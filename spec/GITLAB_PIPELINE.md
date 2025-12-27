# GitLab CI/CD Pipeline

Owner: YOU
Last updated: YYYY-MM-DD

## Objectives
- Automate builds of reCamera OS images using the existing Docker toolchain.
- Run post-build sanity checks (version audit, boot scripts lint).
- Publish OTA-ready artifacts (`*_ota.zip`, `sg2002_recamera_emmc_sha256sum.txt`) as pipeline artifacts.
- Optional hooks for pushing artifacts to the OTA Docker server or external storage.

## Runner Requirements
- GitLab runner with Docker-in-Docker capability. Recommended executor: `docker`.
- Privileged mode enabled (`privileged = true`) to allow nested Docker builds.
- Cache space >= 30 GB (Buildroot toolchain download + output artifacts).

## CI Variables
| Variable | Purpose |
|----------|---------|
| `TARGET` | Build target, default `sg2002_recamera_emmc`. |
| `OTA_VERSION` | Version string for manifest path (e.g., `0.2.1`). |
| `OTA_PUBLISH` | If set to `true`, trigger publish job (push to OTA server/S3). |
| `OTA_SERVER_HOST`/`OTA_SERVER_PORT` | Optional, used by publish stage. |

## Pipeline Stages
1. **prepare** – checkout submodules, setup environment metadata.
2. **build** – run `./docker_build.sh $TARGET` to produce images.
3. **audit** – execute `spec/version_audit.sh` on the build output using appropriate container; capture report.
4. **package** – generate `sg2002_recamera_emmc_sha256sum.txt`, collect OTA zip, and expose as artifacts.
5. **publish** (optional/manual) – push artifacts to OTA server via `curl`/`scp` or other mechanism.

## Sample `.gitlab-ci.yml`
```yaml
stages:
  - prepare
  - build
  - audit
  - package
  - publish

variables:
  TARGET: "sg2002_recamera_emmc"
  DOCKER_DRIVER: overlay2
  GIT_SUBMODULE_STRATEGY: recursive

default:
  image: docker:24.0.5
  services:
    - docker:24.0.5-dind
  before_script:
    - apk add --no-cache bash git make python3 py3-pip coreutils
    - git config --global --add safe.directory "$CI_PROJECT_DIR"

prepare:
  stage: prepare
  script:
    - git submodule update --init --recursive --depth 1
    - ./scripts/git_version_v420 || true
  artifacts:
    expire_in: 2 hrs
    paths:
      - scripts/git_version_*/

build:
  stage: build
  script:
    - chmod +x docker_build.sh
    - ./docker_build.sh "$TARGET"
  artifacts:
    when: always
    expire_in: 1 week
    paths:
      - output/
      - buildroot-2021.05/output/
    reports:
      junit: output/${TARGET}/install/build_junit.xml
  needs: [prepare]

version_audit:
  stage: audit
  image: alpine:3.20
  script:
    - apk add --no-cache bash coreutils
    - cp spec/version_audit.sh /tmp/version_audit.sh
    - chmod +x /tmp/version_audit.sh
    - OUTPUT_FILE=/tmp/version_audit.txt /tmp/version_audit.sh || true
    - cat /tmp/version_audit.txt
  artifacts:
    expire_in: 1 week
    paths:
      - /tmp/version_audit.txt
  needs: [build]

package:
  stage: package
  image: alpine:3.20
  script:
    - apk add --no-cache bash coreutils
    - mkdir -p ota_release/releases/${OTA_VERSION}
    - find output/${TARGET}/install/soc_${TARGET} -name '*_ota.zip' -maxdepth 1 -print -exec cp {} ota_release/releases/${OTA_VERSION}/ \;
    - (cd ota_release/releases/${OTA_VERSION} && sha256sum *.zip > sg2002_recamera_emmc_sha256sum.txt)
    - ls -R ota_release
  artifacts:
    expire_in: 4 weeks
    paths:
      - ota_release/
  needs: [build]

publish:
  stage: publish
  image: alpine:3.20
  rules:
    - if: "$OTA_PUBLISH == \"true\""
      when: on_success
    - when: manual
  script:
    - apk add --no-cache curl openssh-client
    - echo "Publishing OTA artifacts to ${OTA_SERVER_HOST}:${OTA_SERVER_PORT:-8080}"
    # Example curl upload; replace with actual deployment logic
    - |
      if [ -n "$OTA_SERVER_HOST" ]; then
        for f in ota_release/releases/${OTA_VERSION}/*; do
          echo "Uploading $f"
          curl -T "$f" "http://${OTA_SERVER_HOST}:${OTA_SERVER_PORT:-8080}/releases/${OTA_VERSION}/"
        done
      else
        echo "OTA_SERVER_HOST not set; skipping publish"
      fi
  needs: [package]
  dependencies:
    - package
```

## Notes
- Adjust `docker_build.sh` options (e.g., `--shell`) if you need more control inside pipeline.
- Captured artifacts include full `output/` tree; prune if storage is a concern.
- Replace placeholder publish logic with your actual OTA server deployment (SCP to host, S3 upload, etc.).
- Consider adding security scanning (e.g., `trivy`, `container scan`) in a separate stage.

## Next Steps
- Validate `.gitlab-ci.yml` locally via `gitlab-runner exec docker build` or using GitLab CI lint tool.
- Integrate pipeline secrets (e.g., OTA server credentials) via GitLab CI variables.
- Extend pipeline with automated tests (unit tests, boot smoke test with QEMU, etc.).
