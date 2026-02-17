#include "mongoose.h"

#include <signal.h>
#include <stdio.h>
#include <string.h>
#include <sys/stat.h>
#include <errno.h>

static volatile sig_atomic_t g_should_exit = 0;

static void handle_signal(int signo) {
  (void) signo;
  g_should_exit = 1;
}

static void reply_json(struct mg_connection *c, int status, const char *json) {
  mg_http_reply(c, status, "Content-Type: application/json\r\n", "%s\n", json);
}

struct app_config {
  const char *listen_addr;
  const char *root_dir;
  const char *cert_file;
  const char *key_file;
};

static void get_mac_address(const char *interface, char *mac_buf, size_t buf_size) {
  char path[256];
  snprintf(path, sizeof(path), "/sys/class/net/%s/address", interface);
  
  FILE *fp = fopen(path, "r");
  if (fp) {
    if (fgets(mac_buf, buf_size, fp)) {
      // Remove newline
      size_t len = strlen(mac_buf);
      if (len > 0 && mac_buf[len-1] == '\n') {
        mac_buf[len-1] = '\0';
      }
    }
    fclose(fp);
  } else {
    strncpy(mac_buf, "unknown", buf_size);
  }
}

// Helper function to check if URI matches (with or without /oobe prefix)
static int uri_matches(struct mg_str uri, const char *path) {
  // Check exact match
  if (mg_strcmp(uri, mg_str(path)) == 0) return 1;
  
  // Check with /oobe prefix
  char prefixed[256];
  snprintf(prefixed, sizeof(prefixed), "/oobe%s", path);
  if (mg_strcmp(uri, mg_str(prefixed)) == 0) return 1;
  
  return 0;
}

static void fn(struct mg_connection *c, int ev, void *ev_data) {
  if (ev != MG_EV_HTTP_MSG) return;

  struct mg_http_message *hm = (struct mg_http_message *) ev_data;
  const struct app_config *cfg = (const struct app_config *) c->fn_data;

  // API endpoints - handle both with and without /oobe prefix
  // (supervisor proxy strips /oobe prefix, but direct access keeps it)
  if (uri_matches(hm->uri, "/api/health")) {
    reply_json(c, 200, "{\"ok\":true,\"service\":\"oobe\"}");
    return;
  }

  if (uri_matches(hm->uri, "/api/getNetworkInfo")) {
    char eth0_mac[32] = {0};
    char wlan0_mac[32] = {0};
    
    get_mac_address("eth0", eth0_mac, sizeof(eth0_mac));
    get_mac_address("wlan0", wlan0_mac, sizeof(wlan0_mac));
    
    char response[512];
    snprintf(response, sizeof(response),
      "{\"ok\":true,\"interfaces\":{\"eth0\":{\"mac\":\"%s\"},\"wlan0\":{\"mac\":\"%s\"}}}",
      eth0_mac, wlan0_mac);
    
    reply_json(c, 200, response);
    return;
  }

  if (uri_matches(hm->uri, "/api/saveRegistration") && mg_strcmp(hm->method, mg_str("POST")) == 0) {
    // Create userdata/config directory if it doesn't exist
    mkdir("/userdata", 0755);
    mkdir("/userdata/config", 0755);

    // Write registration data to file
    FILE *fp = fopen("/userdata/config/police-record.json", "w");
    if (fp) {
      fwrite(hm->body.buf, 1, hm->body.len, fp);
      fclose(fp);
      
      // Set file permissions to 600 (owner read/write only) for security
      chmod("/userdata/config/police-record.json", 0600);
      
      reply_json(c, 200, "{\"ok\":true,\"message\":\"Registration saved\"}");
    } else {
      char error_msg[256];
      snprintf(error_msg, sizeof(error_msg), "{\"ok\":false,\"error\":\"Failed to save: %s\"}", strerror(errno));
      reply_json(c, 500, error_msg);
    }
    return;
  }

  if (uri_matches(hm->uri, "/api/registrationStatus")) {
    // Check if registration file exists
    struct stat st;
    if (stat("/userdata/config/police-record.json", &st) == 0) {
      // File exists, read and return it
      FILE *fp = fopen("/userdata/config/police-record.json", "r");
      if (fp) {
        fseek(fp, 0, SEEK_END);
        long fsize = ftell(fp);
        fseek(fp, 0, SEEK_SET);
        
        char *content = (char*)malloc(fsize + 1);
        if (content) {
          fread(content, 1, fsize, fp);
          content[fsize] = '\0';
          fclose(fp);
          
          // Build response with registration data
          char response[fsize + 100];
          snprintf(response, sizeof(response), "{\"ok\":true,\"registered\":true,\"data\":%s}", content);
          reply_json(c, 200, response);
          free(content);
        } else {
          fclose(fp);
          reply_json(c, 500, "{\"ok\":false,\"error\":\"Memory allocation failed\"}");
        }
      } else {
        reply_json(c, 500, "{\"ok\":false,\"error\":\"Failed to read registration file\"}");
      }
    } else {
      // File doesn't exist, not registered
      reply_json(c, 200, "{\"ok\":true,\"registered\":false}");
    }
    return;
  }

  if (uri_matches(hm->uri, "/api/complete") && mg_strcmp(hm->method, mg_str("POST")) == 0) {
    // Remove the OOBE flag file to signal completion
    if (unlink("/etc/oobe/flag") == 0) {
      reply_json(c, 200, "{\"ok\":true,\"message\":\"OOBE completed\"}");
    } else {
      char error_msg[256];
      snprintf(error_msg, sizeof(error_msg), "{\"ok\":false,\"error\":\"Failed to remove flag: %s\"}", strerror(errno));
      reply_json(c, 500, error_msg);
    }
    return;
  }

  // Handle root path "/" and /index.html - serve index.html for SPA routing
  // This handles OAuth callbacks like /?code=...&state=... or /index.html?code=...&state=...
  // (after supervisor strips /oobe prefix)
  // Also handle /oobe/ and /oobe paths for direct access
  // Extract path without query string for comparison
  struct mg_str path = hm->uri;
  for (size_t i = 0; i < hm->uri.len; i++) {
    if (hm->uri.buf[i] == '?') {
      path.len = i;
      break;
    }
  }
  
  if (mg_strcmp(path, mg_str("/")) == 0 ||
      mg_strcmp(path, mg_str("/index.html")) == 0 ||
      mg_strcmp(path, mg_str("/oobe/")) == 0 ||
      mg_strcmp(path, mg_str("/oobe")) == 0 ||
      mg_strcmp(path, mg_str("/oobe/index.html")) == 0) {
    // Serve index.html for the root path
    char index_path[512];
    snprintf(index_path, sizeof(index_path), "%s/index.html", cfg->root_dir);
    
    struct mg_http_serve_opts opts;
    memset(&opts, 0, sizeof(opts));
    opts.root_dir = cfg->root_dir;
    opts.extra_headers = "Content-Type: text/html\r\n";
    mg_http_serve_file(c, hm, index_path, &opts);
    return;
  }

  // For other paths, serve from root directory
  struct mg_http_serve_opts opts;
  memset(&opts, 0, sizeof(opts));
  opts.root_dir = cfg->root_dir;
  opts.page404 = "index.html";
  
  // If path starts with /oobe/, strip the prefix
  if (hm->uri.len > 6 && strncmp(hm->uri.buf, "/oobe/", 6) == 0) {
    struct mg_http_message modified_hm = *hm;
    modified_hm.uri.buf = hm->uri.buf + 5;  // Skip "/oobe"
    modified_hm.uri.len = hm->uri.len - 5;
    mg_http_serve_dir(c, &modified_hm, &opts);
  } else {
    mg_http_serve_dir(c, hm, &opts);
  }
}

static void usage(const char *argv0) {
  fprintf(stderr, "Usage: %s [OPTIONS]\n", argv0);
  fprintf(stderr, "Options:\n");
  fprintf(stderr, "  --listen URL         Listen address (default: https://0.0.0.0:8081)\n");
  fprintf(stderr, "  --root PATH          Web root directory (default: /usr/share/oobe/www)\n");
  fprintf(stderr, "  --cert PATH          TLS certificate file (default: /etc/supervisor/certs/cert.pem)\n");
  fprintf(stderr, "  --key PATH           TLS key file (default: /etc/supervisor/certs/key.pem)\n");
  fprintf(stderr, "  -h, --help           Show this help\n");
}

int main(int argc, char **argv) {
  struct app_config cfg;
  cfg.listen_addr = "https://0.0.0.0:8081";
  cfg.root_dir = "/usr/share/oobe/www";
  cfg.cert_file = "/etc/supervisor/certs/cert.pem";
  cfg.key_file = "/etc/supervisor/certs/key.pem";

  for (int i = 1; i < argc; i++) {
    if (strcmp(argv[i], "--listen") == 0 && i + 1 < argc) {
      cfg.listen_addr = argv[++i];
    } else if (strcmp(argv[i], "--root") == 0 && i + 1 < argc) {
      cfg.root_dir = argv[++i];
    } else if (strcmp(argv[i], "--cert") == 0 && i + 1 < argc) {
      cfg.cert_file = argv[++i];
    } else if (strcmp(argv[i], "--key") == 0 && i + 1 < argc) {
      cfg.key_file = argv[++i];
    } else if (strcmp(argv[i], "-h") == 0 || strcmp(argv[i], "--help") == 0) {
      usage(argv[0]);
      return 0;
    } else {
      fprintf(stderr, "Unknown arg: %s\n", argv[i]);
      usage(argv[0]);
      return 2;
    }
  }

  signal(SIGINT, handle_signal);
  signal(SIGTERM, handle_signal);

  struct mg_mgr mgr;
  mg_mgr_init(&mgr);

  struct mg_connection *lc = mg_http_listen(&mgr, cfg.listen_addr, fn, &cfg);
  if (lc == NULL) {
    fprintf(stderr, "Failed to listen on %s\n", cfg.listen_addr);
    mg_mgr_free(&mgr);
    return 1;
  }

  // Enable TLS if using https://
  if (strncmp(cfg.listen_addr, "https://", 8) == 0) {
    struct mg_tls_opts tls_opts;
    memset(&tls_opts, 0, sizeof(tls_opts));
    tls_opts.cert = mg_str(cfg.cert_file);
    tls_opts.key = mg_str(cfg.key_file);
    
    mg_tls_init(lc, &tls_opts);
    fprintf(stderr, "OOBE server listening on %s (HTTPS, root: %s)\n", cfg.listen_addr, cfg.root_dir);
    fprintf(stderr, "Using TLS cert: %s\n", cfg.cert_file);
  } else {
    fprintf(stderr, "OOBE server listening on %s (HTTP, root: %s)\n", cfg.listen_addr, cfg.root_dir);
  }

  while (!g_should_exit) {
    mg_mgr_poll(&mgr, 250);
  }

  mg_mgr_free(&mgr);
  return 0;
}
