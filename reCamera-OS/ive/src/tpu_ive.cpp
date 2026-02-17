#include <sys/time.h>
#include <iostream>
#include <memory>
#include "cvi_tpu_ive.h"
#include "ive_internal.hpp"

/**
 * @brief IVE version info
 *
 */
const std::string g_ive_version = std::string(
    std::string("tpu-ive") + "_" +
    std::regex_replace(std::string(__DATE__), std::regex{" "}, std::string{"-"}) + "-" + __TIME__);
#define IVE_VERSION g_ive_version.c_str()

////////////////////////////////////////
IVE_IMAGE_S g_tmp_sub_img[4];

const char *cviTPUIveImgEnTypeStr[] = {
    STRFY(IVE_IMAGE_TYPE_U8C1),         STRFY(IVE_IMAGE_TYPE_S8C1),
    STRFY(IVE_IMAGE_TYPE_YUV420SP),     STRFY(IVE_IMAGE_TYPE_YUV422SP),
    STRFY(IVE_IMAGE_TYPE_YUV420P),      STRFY(IVE_IMAGE_TYPE_YUV422P),
    STRFY(IVE_IMAGE_TYPE_S8C2_PACKAGE), STRFY(IVE_IMAGE_TYPE_S8C2_PLANAR),
    STRFY(IVE_IMAGE_TYPE_S16C1),        STRFY(IVE_IMAGE_TYPE_U16C1),
    STRFY(IVE_IMAGE_TYPE_U8C3_PACKAGE), STRFY(IVE_IMAGE_TYPE_U8C3_PLANAR),
    STRFY(IVE_IMAGE_TYPE_S32C1),        STRFY(IVE_IMAGE_TYPE_U32C1),
    STRFY(IVE_IMAGE_TYPE_S64C1),        STRFY(IVE_IMAGE_TYPE_U64C1),
    STRFY(IVE_IMAGE_TYPE_BF16C1),       STRFY(IVE_IMAGE_TYPE_FP32C1)};

CVI_S32 TPU_IVE_CreateImage(IVE_HANDLE pIveHandle, IVE_IMAGE_S *pstImg, IVE_IMAGE_TYPE_E enType,
                            CVI_U32 u32Width, CVI_U32 u32Height);

CVI_S32 TPU_SYS_FreeI(IVE_HANDLE pIveHandle, IVE_IMAGE_S *pstImg);

IVE_HANDLE CVI_TPU_IVE_CreateHandle() {
  IVE_HANDLE_CTX *handle_ctx = new IVE_HANDLE_CTX;
  if (createHandle(&handle_ctx->rt_handle, &handle_ctx->cvk_ctx) != CVI_SUCCESS) {
    LOGE("Create handle failed.\n");
    delete handle_ctx;
    return NULL;
  }
  if (handle_ctx->t_h.t_tblmgr.init(handle_ctx->rt_handle, handle_ctx->cvk_ctx) != CVI_SUCCESS) {
    LOGE("Create table failed.\n");
    delete handle_ctx;
    return NULL;
  }
  ////////////////////////////////////////////////////
  std::cout << "create image in CreateHandle\n";

  struct timeval t0, t1;
  gettimeofday(&t0, NULL);
  TPU_IVE_CreateImage(handle_ctx, &g_tmp_sub_img[0], IVE_IMAGE_TYPE_U8C1, 32, 2000);
  TPU_IVE_CreateImage(handle_ctx, &g_tmp_sub_img[1], IVE_IMAGE_TYPE_U8C1, 32, 2000);
  TPU_IVE_CreateImage(handle_ctx, &g_tmp_sub_img[2], IVE_IMAGE_TYPE_U8C1, 32, 1000);
  TPU_IVE_CreateImage(handle_ctx, &g_tmp_sub_img[3], IVE_IMAGE_TYPE_U8C1, 32, 1000);
  gettimeofday(&t1, NULL);
  unsigned long elapsed_cpu = (t1.tv_sec - t0.tv_sec) * 1000000 + t1.tv_usec - t0.tv_usec;
  printf("Create image in CreateHandle, elapsed: %lu us\n", elapsed_cpu);
  //////////////////////////////////////////////////
  LOGI("IVE_HANDLE created, version %s", IVE_VERSION);
  return (void *)handle_ctx;
}

CVI_S32 CVI_TPU_IVE_DestroyHandle(IVE_HANDLE pIveHandle) {
  /////////////////////////////////////////////////
  TPU_SYS_FreeI(pIveHandle, &g_tmp_sub_img[0]);
  TPU_SYS_FreeI(pIveHandle, &g_tmp_sub_img[1]);
  TPU_SYS_FreeI(pIveHandle, &g_tmp_sub_img[2]);
  TPU_SYS_FreeI(pIveHandle, &g_tmp_sub_img[3]);

  // delete[] g_tmp_sub_img;

  IVE_HANDLE_CTX *handle_ctx = reinterpret_cast<IVE_HANDLE_CTX *>(pIveHandle);
  handle_ctx->t_h.t_tblmgr.free(handle_ctx->rt_handle);
  destroyHandle(handle_ctx->rt_handle, handle_ctx->cvk_ctx);
  delete handle_ctx;
  LOGI("Destroy handle.\n");

  return CVI_SUCCESS;
}

CVI_S32 TPU_Free_Image(IVE_IMAGE_S *pstImg) {
  if (pstImg->tpu_block != nullptr) {
    CviImg *p_cvi_img = reinterpret_cast<CviImg *>(pstImg->tpu_block);
    delete p_cvi_img;
  }
  return CVI_SUCCESS;
}

static void ViewAsYuv420(IVE_IMAGE_S *src) {
  int w = src->u16Stride[0];
  int w1 = src->u16Stride[1];
  int w2 = src->u16Stride[1];
  int h = src->u16Height;
  int h2 = src->u16Height / 2;

  memcpy(src->pu8VirAddr[2], src->pu8VirAddr[0] + w * h + w1 * h2, w2 * h2);
  memcpy(src->pu8VirAddr[1], src->pu8VirAddr[0] + w * h, w1 * h2);
}

static CviImg *ViewAsU8C1(IVE_IMAGE_S *src) {
  CVIIMGTYPE img_type = CVIIMGTYPE::CVI_GRAY;
  cvk_fmt_t fmt = CVK_FMT_U8;
  std::vector<uint32_t> heights;
  uint16_t new_height;
  CviImg *orig_cpp = reinterpret_cast<CviImg *>(src->tpu_block);

  if (src->enType == IVE_IMAGE_TYPE_YUV420P) {
    int halfh = src->u16Height / 2;
    int u_plane_size = src->u16Stride[1] * halfh;
    int v_plane_size = src->u16Stride[2] * halfh;
    int added_h = (u_plane_size + v_plane_size) / src->u16Stride[0];
    LOGD("ViewAsU8C1 stride0:%d,%d,%d,addedh:%d\n", (int)src->u16Stride[0], (int)src->u16Stride[1],
         (int)src->u16Stride[2], added_h);
    new_height = src->u16Height + added_h;

    if (orig_cpp->IsSubImg()) {
      std::copy(src->pu8VirAddr[1], src->pu8VirAddr[1] + u_plane_size,
                src->pu8VirAddr[0] + src->u16Stride[0] * src->u16Height);
      std::copy(src->pu8VirAddr[2], src->pu8VirAddr[2] + v_plane_size,
                src->pu8VirAddr[0] + src->u16Stride[0] * src->u16Height + u_plane_size);
    }
  } else if (src->enType == IVE_IMAGE_TYPE_U8C3_PLANAR) {
    new_height = src->u16Height * 3;
  } else {
    LOGE("Only support IVE_IMAGE_TYPE_YUV420P or IVE_IMAGE_TYPE_U8C3_PLANAR.\n");
    return nullptr;
  }

  heights.push_back(new_height);

  std::vector<uint32_t> strides, u32_length;
  strides.push_back(src->u16Stride[0]);

  if (Is4096Workaound(orig_cpp->GetImgType())) {
    u32_length.push_back(orig_cpp->GetImgCOffsets()[3]);
  } else {
    u32_length.push_back(src->u16Stride[0] * new_height);
  }

  auto *cpp_img = new CviImg(new_height, src->u16Width, strides, heights, u32_length,
                             src->pu8VirAddr[0], src->u64PhyAddr[0], img_type, fmt);
  if (!cpp_img->IsInit()) {
    LOGE("Failed to init IVE_IMAGE_S.\n");
    delete cpp_img;
    return nullptr;
  }
  return cpp_img;
}
CVI_S32 CVI_IVE_Blend_Pixel(IVE_HANDLE pIveHandle, IVE_SRC_IMAGE_S *pstSrc1,
                            IVE_SRC_IMAGE_S *pstSrc2, IVE_SRC_IMAGE_S *pstAlpha,
                            IVE_DST_IMAGE_S *pstDst, bool bInstant) {
  ScopedTrace t(__PRETTY_FUNCTION__);
  if (!IsValidImageType(pstSrc1, STRFY(pstSrc1), IVE_IMAGE_TYPE_U8C1, IVE_IMAGE_TYPE_U8C3_PLANAR,
                        IVE_IMAGE_TYPE_YUV420P)) {
    LOGE(
        "image type of pstSrc1 should be one of (IVE_IMAGE_TYPE_U8C1, "
        "IVE_IMAGE_TYPE_U8C3_PLANAR)\n");
    return CVI_FAILURE;
  }
  if (!IsValidImageType(pstSrc2, STRFY(pstSrc2), IVE_IMAGE_TYPE_U8C1, IVE_IMAGE_TYPE_U8C3_PLANAR,
                        IVE_IMAGE_TYPE_YUV420P)) {
    LOGE(
        "image type of pstSrc2 should be one of (IVE_IMAGE_TYPE_U8C1, "
        "IVE_IMAGE_TYPE_U8C3_PLANAR)\n");
    return CVI_FAILURE;
  }
  if (!IsValidImageType(pstAlpha, STRFY(pstAlpha), IVE_IMAGE_TYPE_U8C1, IVE_IMAGE_TYPE_U8C3_PLANAR,
                        IVE_IMAGE_TYPE_YUV420P)) {
    LOGE(
        "image type of pstDst should be one of (IVE_IMAGE_TYPE_U8C1, "
        "IVE_IMAGE_TYPE_U8C3_PLANAR)\n");
    return CVI_FAILURE;
  }
  if (!IsValidImageType(pstDst, STRFY(pstDst), IVE_IMAGE_TYPE_U8C1, IVE_IMAGE_TYPE_U8C3_PLANAR,
                        IVE_IMAGE_TYPE_YUV420P)) {
    LOGE(
        "image type of pstDst should be one of (IVE_IMAGE_TYPE_U8C1, "
        "IVE_IMAGE_TYPE_U8C3_PLANAR) \n");
    return CVI_FAILURE;
  }

  if ((pstDst->enType != pstSrc1->enType) || (pstDst->enType != pstSrc2->enType) ||
      (pstDst->enType != pstAlpha->enType)) {
    LOGE("source1/source2/dst image pixel format do not match,%d,%d,%d!\n", pstSrc1->enType,
         pstSrc2->enType, pstDst->enType);
    return CVI_FAILURE;
  }

  int ret = CVI_FAILURE;
  IVE_HANDLE_CTX *handle_ctx = reinterpret_cast<IVE_HANDLE_CTX *>(pIveHandle);

  std::shared_ptr<CviImg> cpp_src1;
  std::shared_ptr<CviImg> cpp_src2;
  std::shared_ptr<CviImg> cpp_alpha;
  std::shared_ptr<CviImg> cpp_dst;

  if (pstDst->enType == IVE_IMAGE_TYPE_YUV420P) {
    // NOTE: Computing tpu slice with different stride in different channel is quite complicated.
    // Instead, we consider YUV420P image as U8C1 with Wx(H + H / 2) image size so that there is
    // only one channel have to blended.
    cpp_src1 = std::shared_ptr<CviImg>(ViewAsU8C1(pstSrc1));
    cpp_src2 = std::shared_ptr<CviImg>(ViewAsU8C1(pstSrc2));
    cpp_alpha = std::shared_ptr<CviImg>(ViewAsU8C1(pstAlpha));
    cpp_dst = std::shared_ptr<CviImg>(ViewAsU8C1(pstDst));
  } else {
    cpp_src1 =
        std::shared_ptr<CviImg>(reinterpret_cast<CviImg *>(pstSrc1->tpu_block), [](CviImg *) {});
    cpp_src2 =
        std::shared_ptr<CviImg>(reinterpret_cast<CviImg *>(pstSrc2->tpu_block), [](CviImg *) {});
    cpp_alpha =
        std::shared_ptr<CviImg>(reinterpret_cast<CviImg *>(pstAlpha->tpu_block), [](CviImg *) {});
    cpp_dst =
        std::shared_ptr<CviImg>(reinterpret_cast<CviImg *>(pstDst->tpu_block), [](CviImg *) {});
  }

  if (cpp_src1 == nullptr || cpp_src2 == nullptr || cpp_alpha == nullptr || cpp_dst == nullptr) {
    LOGE("Cannot get tpu block\n");
    return CVI_FAILURE;
  }

  if ((cpp_src1->GetImgHeight() != cpp_src2->GetImgHeight()) ||
      (cpp_src1->GetImgHeight() != cpp_dst->GetImgHeight()) ||
      (cpp_src1->GetImgWidth() != cpp_src2->GetImgWidth()) ||
      (cpp_src1->GetImgWidth() != cpp_dst->GetImgWidth()) ||
      (cpp_src1->GetImgWidth() != cpp_alpha->GetImgWidth()) ||
      (cpp_src1->GetImgHeight() != cpp_alpha->GetImgHeight()) ||
      (cpp_src1->GetImgChannel() != cpp_src2->GetImgChannel()) ||
      (cpp_src1->GetImgChannel() != cpp_alpha->GetImgChannel()) ||
      (cpp_src1->GetImgChannel() != cpp_dst->GetImgChannel())) {
    LOGE("source1/source2/alpha/dst image size do not match!\n");
    return CVI_FAILURE;
  }

  std::vector<CviImg *> inputs = {cpp_src1.get(), cpp_src2.get(), cpp_alpha.get()};
  std::vector<CviImg *> outputs = {cpp_dst.get()};

  handle_ctx->t_h.t_blend_pixel.init(handle_ctx->rt_handle, handle_ctx->cvk_ctx);
  ret = handle_ctx->t_h.t_blend_pixel.run(handle_ctx->rt_handle, handle_ctx->cvk_ctx, inputs,
                                          outputs);
  if (pstDst->enType == IVE_IMAGE_TYPE_YUV420P) {
    CviImg *orig_cpp = reinterpret_cast<CviImg *>(pstDst->tpu_block);
    if (orig_cpp->IsSubImg()) {
      ViewAsYuv420(pstDst);
    }
  }
  return ret;
}

CVI_S32 CVI_IVE_ImageInit(IVE_IMAGE_S *pstSrc) {
  if (pstSrc->tpu_block != NULL) {
    return CVI_SUCCESS;
  }
  size_t c = 1;
  CVIIMGTYPE img_type = CVIIMGTYPE::CVI_GRAY;
  cvk_fmt_t fmt = CVK_FMT_U8;
  std::vector<uint32_t> heights;
  switch (pstSrc->enType) {
    case IVE_IMAGE_TYPE_U8C1: {
      heights.push_back(pstSrc->u16Height);
    } break;
    case IVE_IMAGE_TYPE_S8C1: {
      heights.push_back(pstSrc->u16Height);
      fmt = CVK_FMT_I8;
    } break;
    case IVE_IMAGE_TYPE_YUV420SP: {
      c = 2;
      img_type = CVIIMGTYPE::CVI_YUV420SP;
      heights.push_back(pstSrc->u16Height);
      heights.push_back(pstSrc->u16Height >> 1);
    } break;
    case IVE_IMAGE_TYPE_YUV420P: {
      c = 3;
      img_type = CVIIMGTYPE::CVI_YUV420P;
      heights.push_back(pstSrc->u16Height);
      heights.push_back(pstSrc->u16Height >> 1);
      heights.push_back(pstSrc->u16Height >> 1);
    } break;
    case IVE_IMAGE_TYPE_YUV422P: {
      c = 3;
      img_type = CVIIMGTYPE::CVI_YUV422P;
      heights.resize(3, pstSrc->u16Height);
    } break;
    case IVE_IMAGE_TYPE_U8C3_PACKAGE: {
      c = 1;
      img_type = CVIIMGTYPE::CVI_RGB_PACKED;
      heights.push_back(pstSrc->u16Height);
    } break;
    case IVE_IMAGE_TYPE_S8C3_PACKAGE: {
      c = 1;
      img_type = CVIIMGTYPE::CVI_RGB_PACKED;
      heights.push_back(pstSrc->u16Height);
      fmt = CVK_FMT_I8;
    } break;
    case IVE_IMAGE_TYPE_U8C3_PLANAR: {
      c = 3;
      img_type = CVIIMGTYPE::CVI_RGB_PLANAR;
      heights.resize(c, pstSrc->u16Height);
    } break;
    case IVE_IMAGE_TYPE_S8C3_PLANAR: {
      c = 3;
      img_type = CVIIMGTYPE::CVI_RGB_PLANAR;
      heights.resize(c, pstSrc->u16Height);
      fmt = CVK_FMT_I8;
    } break;
    default: {
      LOGE("Unsupported conversion type: %u.\n", pstSrc->enType);
      return CVI_FAILURE;
    } break;
  }
  std::vector<uint32_t> strides, u32_length;
  for (size_t i = 0; i < c; i++) {
    strides.push_back(pstSrc->u16Stride[i]);
    u32_length.push_back(pstSrc->u16Stride[i] * heights[i]);
  }
  auto *cpp_img = new CviImg(pstSrc->u16Height, pstSrc->u16Width, strides, heights, u32_length,
                             pstSrc->pu8VirAddr[0], pstSrc->u64PhyAddr[0], img_type, fmt);
  if (!cpp_img->IsInit()) {
    LOGE("Failed to init IVE_IMAGE_S.\n");
    return CVI_FAILURE;
  }

  pstSrc->tpu_block = reinterpret_cast<CVI_IMG *>(cpp_img);
  return CVI_SUCCESS;
}

// flag,if 0,extract Y plane, if 1 extract UV plane
CVI_S32 VideoFrameYInfo2Image(VIDEO_FRAME_INFO_S *pstVFISrc, IVE_IMAGE_S *pstIIDst, int flag) {
  memset(pstIIDst, 0, sizeof(IVE_IMAGE_S));
  CviImg *cpp_img = nullptr;
  if (pstIIDst->tpu_block != NULL) {
    cpp_img = reinterpret_cast<CviImg *>(pstIIDst->tpu_block);
    if (!cpp_img->IsNullMem()) {
      LOGE("pstIIDst->tpu_block->m_rtmem is not NULL");
      return CVI_FAILURE;
    }
    if (cpp_img->GetMagicNum() != CVI_IMG_VIDEO_FRM_MAGIC_NUM) {
      printf("pstIIDst->tpu_block is not constructed from VIDEO_FRAME_INFO_S");
      return CVI_FAILURE;
    }
  }
  VIDEO_FRAME_S *pstVFSrc = &pstVFISrc->stVFrame;

  CVIIMGTYPE img_type = CVIIMGTYPE::CVI_GRAY;
  cvk_fmt_t fmt = CVK_FMT_U8;

  std::vector<uint32_t> strides, u32_length, heights;
  uint32_t img_w = pstVFSrc->u32Width;
  uint32_t img_h = pstVFSrc->u32Height;

  uint8_t *vaddr = pstVFSrc->pu8VirAddr[0];
  uint64_t paddr = pstVFSrc->u64PhyAddr[0];

  if (flag == 0 && (pstVFSrc->enPixelFormat == PIXEL_FORMAT_NV21 ||
                    pstVFSrc->enPixelFormat == PIXEL_FORMAT_NV12 ||
                    pstVFSrc->enPixelFormat == PIXEL_FORMAT_YUV_PLANAR_420 ||
                    pstVFSrc->enPixelFormat == PIXEL_FORMAT_YUV_400)) {
    heights.push_back(pstVFSrc->u32Height);
    strides.push_back(pstVFSrc->u32Stride[0]);
    u32_length.push_back(pstVFSrc->u32Length[0]);
  } else if (flag == 1 && (pstVFSrc->enPixelFormat == PIXEL_FORMAT_NV21 ||
                           pstVFSrc->enPixelFormat == PIXEL_FORMAT_NV12)) {
    vaddr = pstVFSrc->pu8VirAddr[1];
    paddr = pstVFSrc->u64PhyAddr[1];
    if (paddr - pstVFSrc->u64PhyAddr[0] != pstVFSrc->u32Length[0]) {
      LOGE("error,plane1 addr %ld,plane0 addr:%ld,plane1 length:%d\n", pstVFSrc->u64PhyAddr[1],
           pstVFSrc->u64PhyAddr[0], pstVFSrc->u32Length[0]);
    }
    img_h = pstVFSrc->u32Height / 2;
    heights.push_back(pstVFSrc->u32Height / 2);
    strides.push_back(pstVFSrc->u32Stride[1]);
    u32_length.push_back(pstVFSrc->u32Length[1]);
  } else {
    LOGE("error,img type:%d,flag:%d\n", pstVFSrc->enPixelFormat, flag);
    return CVI_FAILURE;
  }

  if (cpp_img == nullptr) {
    // LOGD("init CviImg\n");
    cpp_img = new CviImg(img_h, img_w, strides, heights, u32_length, vaddr, paddr, img_type, fmt);
  } else {
    cpp_img->ReInit(img_h, img_w, strides, heights, u32_length, vaddr, paddr, img_type, fmt);
  }

  if (!cpp_img->IsInit()) {
    LOGE("Failed to init IVE_IMAGE_S.\n");
    return CVI_FAILURE;
  }

  pstIIDst->enType = IVE_IMAGE_TYPE_U8C1;
  pstIIDst->tpu_block = reinterpret_cast<CVI_IMG *>(cpp_img);
  pstIIDst->u16Width = cpp_img->GetImgWidth();
  pstIIDst->u16Height = cpp_img->GetImgHeight();
  pstIIDst->u16Reserved = getFmtSize(fmt);

  size_t i_limit = cpp_img->GetImgChannel();

  // LOGD("extract y plane image,num_plane:%d\n",i_limit);
  for (size_t i = 0; i < i_limit; i++) {
    pstIIDst->pu8VirAddr[i] = cpp_img->GetVAddr() + cpp_img->GetImgCOffsets()[i];
    pstIIDst->u64PhyAddr[i] = cpp_img->GetPAddr() + cpp_img->GetImgCOffsets()[i];
    pstIIDst->u16Stride[i] = cpp_img->GetImgStrides()[i];
    // LOGD("imgoffsets:%u,stride:%u\n",cpp_img->GetImgCOffsets()[i],cpp_img->GetImgStrides()[i]);
  }

  for (size_t i = i_limit; i < 3; i++) {
    pstIIDst->pu8VirAddr[i] = NULL;
    pstIIDst->u64PhyAddr[i] = 0;
    pstIIDst->u16Stride[i] = 0;
  }
  return CVI_SUCCESS;
}

CVI_S32 TPU_IVE_VideoFrameInfo2Image(VIDEO_FRAME_INFO_S *pstVFISrc, IVE_IMAGE_S *pstIIDst) {
  memset(pstIIDst, 0, sizeof(IVE_IMAGE_S));
  CviImg *cpp_img = nullptr;
  if (pstIIDst->tpu_block != NULL) {
    cpp_img = reinterpret_cast<CviImg *>(pstIIDst->tpu_block);
    if (!cpp_img->IsNullMem()) {
      LOGE("pstIIDst->tpu_block->m_rtmem is not NULL");
      return CVI_FAILURE;
    }
    if (cpp_img->GetMagicNum() != CVI_IMG_VIDEO_FRM_MAGIC_NUM) {
      printf("pstIIDst->tpu_block is not constructed from VIDEO_FRAME_INFO_S");
      return CVI_FAILURE;
    }
  }
  VIDEO_FRAME_S *pstVFSrc = &pstVFISrc->stVFrame;
  size_t c = 1;
  CVIIMGTYPE img_type = CVIIMGTYPE::CVI_GRAY;
  cvk_fmt_t fmt = CVK_FMT_U8;
  std::vector<uint32_t> heights;
  switch (pstVFSrc->enPixelFormat) {
    case PIXEL_FORMAT_YUV_400: {
      pstIIDst->enType = IVE_IMAGE_TYPE_U8C1;
      heights.push_back(pstVFSrc->u32Height);
    } break;
    case PIXEL_FORMAT_NV21:
    case PIXEL_FORMAT_NV12: {
      c = 2;
      img_type = CVIIMGTYPE::CVI_YUV420SP;
      pstIIDst->enType = IVE_IMAGE_TYPE_YUV420SP;
      heights.push_back(pstVFSrc->u32Height);
      heights.push_back(pstVFSrc->u32Height >> 1);
    } break;
    case PIXEL_FORMAT_YUV_PLANAR_420: {
      c = 3;
      img_type = CVIIMGTYPE::CVI_YUV420P;
      pstIIDst->enType = IVE_IMAGE_TYPE_YUV420P;
      heights.push_back(pstVFSrc->u32Height);
      heights.push_back(pstVFSrc->u32Height >> 1);
      heights.push_back(pstVFSrc->u32Height >> 1);
    } break;
    case PIXEL_FORMAT_YUV_PLANAR_422: {
      c = 3;
      img_type = CVIIMGTYPE::CVI_YUV422P;
      pstIIDst->enType = IVE_IMAGE_TYPE_YUV422P;
      heights.resize(3, pstVFSrc->u32Height);
    } break;
    case PIXEL_FORMAT_RGB_888:
    case PIXEL_FORMAT_BGR_888: {
      c = 1;
      img_type = CVIIMGTYPE::CVI_RGB_PACKED;
      pstIIDst->enType = IVE_IMAGE_TYPE_U8C3_PACKAGE;
      heights.push_back(pstVFSrc->u32Height);
    } break;
    case PIXEL_FORMAT_RGB_888_PLANAR: {
      c = 3;
      img_type = CVIIMGTYPE::CVI_RGB_PLANAR;
      pstIIDst->enType = IVE_IMAGE_TYPE_U8C3_PLANAR;
      heights.resize(c, pstVFSrc->u32Height);
    } break;
    default: {
      LOGE("Unsupported conversion type: %u.\n", pstVFSrc->enPixelFormat);
      return CVI_FAILURE;
    } break;
  }
  std::vector<uint32_t> strides, u32_length;
  for (size_t i = 0; i < c; i++) {
    strides.push_back(pstVFSrc->u32Stride[i]);
    u32_length.push_back(pstVFSrc->u32Length[i]);
  }
  if (cpp_img == nullptr) {
    cpp_img = new CviImg(pstVFSrc->u32Height, pstVFSrc->u32Width, strides, heights, u32_length,
                         pstVFSrc->pu8VirAddr[0], pstVFSrc->u64PhyAddr[0], img_type, fmt);
  } else {
    cpp_img->ReInit(pstVFSrc->u32Height, pstVFSrc->u32Width, strides, heights, u32_length,
                    pstVFSrc->pu8VirAddr[0], pstVFSrc->u64PhyAddr[0], img_type, fmt);
  }

  if (!cpp_img->IsInit()) {
    LOGE("Failed to init IVE_IMAGE_S.\n");
    return CVI_FAILURE;
  }

  pstIIDst->tpu_block = reinterpret_cast<CVI_IMG *>(cpp_img);
  pstIIDst->u16Width = cpp_img->GetImgWidth();
  pstIIDst->u16Height = cpp_img->GetImgHeight();
  pstIIDst->u16Reserved = getFmtSize(fmt);

  size_t i_limit = cpp_img->GetImgChannel();
  for (size_t i = 0; i < i_limit; i++) {
    pstIIDst->pu8VirAddr[i] = cpp_img->GetVAddr() + cpp_img->GetImgCOffsets()[i];
    pstIIDst->u64PhyAddr[i] = cpp_img->GetPAddr() + cpp_img->GetImgCOffsets()[i];
    pstIIDst->u16Stride[i] = cpp_img->GetImgStrides()[i];
  }

  for (size_t i = i_limit; i < 3; i++) {
    pstIIDst->pu8VirAddr[i] = NULL;
    pstIIDst->u64PhyAddr[i] = 0;
    pstIIDst->u16Stride[i] = 0;
  }
  return CVI_SUCCESS;
}

CVI_S32 CVI_IVE_Blend_Pixel_Y(IVE_HANDLE pIveHandle, VIDEO_FRAME_INFO_S *pstSrc1,
                              VIDEO_FRAME_INFO_S *pstSrc2_dst, VIDEO_FRAME_INFO_S *pstAlpha) {
  IVE_IMAGE_S src1, src2, alpha, dst;

  CVI_S32 ret = CVI_SUCCESS;
  ret = VideoFrameYInfo2Image(pstSrc1, &src1, 0);
  if (ret != CVI_SUCCESS) {
    LOGE("pstSrc1 type not supported,could not extract Y plane");
    return ret;
  }

  ret = VideoFrameYInfo2Image(pstSrc2_dst, &src2, 0);
  if (ret != CVI_SUCCESS) {
    LOGE("pstSrc2_dst type not supported,could not extract Y plane");
    return ret;
  }

  ret = VideoFrameYInfo2Image(pstAlpha, &alpha, 0);
  if (ret != CVI_SUCCESS) {
    LOGE("pstAlpha type not supported,could not extract Y plane");
    return ret;
  }

  ret = VideoFrameYInfo2Image(pstSrc2_dst, &dst, 0);
  if (ret != CVI_SUCCESS) {
    LOGE("pstSrc2 type not supported,could not extract Y plane");
    return ret;
  }

  CVI_IVE_Blend_Pixel(pIveHandle, &src1, &src2, &alpha, &dst, true);

  TPU_Free_Image(&src1);
  TPU_Free_Image(&src2);
  TPU_Free_Image(&alpha);
  TPU_Free_Image(&dst);

  return ret;
}

CVI_S32 TPU_IVE_CreateImage2(IVE_HANDLE pIveHandle, IVE_IMAGE_S *pstImg, IVE_IMAGE_TYPE_E enType,
                             uint32_t u32Width, uint32_t u32Height, IVE_IMAGE_S *pstBuffer) {
  if (u32Width == 0 || u32Height == 0) {
    LOGE("Image width or height cannot be 0.\n");
    pstImg->tpu_block = NULL;
    pstImg->enType = enType;
    pstImg->u16Width = 0;
    pstImg->u16Height = 0;
    pstImg->u16Reserved = 0;
    for (size_t i = 0; i < 3; i++) {
      pstImg->pu8VirAddr[i] = NULL;
      pstImg->u64PhyAddr[i] = 0;
      pstImg->u16Stride[i] = 0;
    }
    return CVI_FAILURE;
  }
  IVE_HANDLE_CTX *handle_ctx = reinterpret_cast<IVE_HANDLE_CTX *>(pIveHandle);
  int fmt_size = 1;
  cvk_fmt_t fmt = CVK_FMT_U8;
  CVIIMGTYPE img_type;
  std::vector<uint32_t> strides;
  std::vector<uint32_t> heights;
  switch (enType) {
    case IVE_IMAGE_TYPE_S8C1: {
      img_type = CVI_GRAY;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN);
      strides.push_back(stride);
      heights.push_back(u32Height);
      fmt = CVK_FMT_I8;
    } break;
    case IVE_IMAGE_TYPE_U8C1: {
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN);
      strides.push_back(stride);
      heights.push_back(u32Height);
      img_type = CVI_GRAY;
    } break;
    case IVE_IMAGE_TYPE_YUV420SP: {
      img_type = CVI_YUV420SP;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN);
      strides.push_back(stride);
      strides.push_back(stride);
      heights.push_back(u32Height);
      heights.push_back(u32Height >> 1);
    } break;
    case IVE_IMAGE_TYPE_YUV420P: {
      img_type = CVI_YUV420P;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN);
      strides.push_back(stride);
      const uint32_t stride2 = WidthAlign(u32Width >> 1, DEFAULT_ALIGN);
      strides.push_back(stride2);
      strides.push_back(stride2);
      heights.push_back(u32Height);
      heights.push_back(u32Height >> 1);
      heights.push_back(u32Height >> 1);
    } break;
    case IVE_IMAGE_TYPE_YUV422P: {
      img_type = CVI_YUV422P;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN);
      const uint32_t stride2 = WidthAlign(u32Width >> 1, DEFAULT_ALIGN);
      strides.push_back(stride);
      strides.push_back(stride2);
      strides.push_back(stride2);
      heights.resize(3, u32Height);
    } break;
    case IVE_IMAGE_TYPE_U8C3_PACKAGE: {
      img_type = CVI_RGB_PACKED;
      const uint32_t stride = WidthAlign(u32Width * 3, DEFAULT_ALIGN);
      strides.push_back(stride);
      heights.push_back(u32Height);
    } break;
    case IVE_IMAGE_TYPE_S8C3_PACKAGE: {
      img_type = CVI_RGB_PACKED;
      const uint32_t stride = WidthAlign(u32Width * 3, DEFAULT_ALIGN);
      strides.push_back(stride);
      heights.push_back(u32Height);
      fmt = CVK_FMT_I8;
    } break;
    case IVE_IMAGE_TYPE_U8C3_PLANAR: {
      img_type = CVI_RGB_PLANAR;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN);
      strides.resize(3, stride);
      heights.resize(3, u32Height);
    } break;
    case IVE_IMAGE_TYPE_S8C3_PLANAR: {
      img_type = CVI_RGB_PLANAR;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN);
      strides.resize(3, stride);
      heights.resize(3, u32Height);
      fmt = CVK_FMT_I8;
    } break;
    case IVE_IMAGE_TYPE_BF16C1: {
      img_type = CVI_SINGLE;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN) * sizeof(int16_t);
      strides.push_back(stride);
      heights.push_back(u32Height);
      fmt_size = 2;
      fmt = CVK_FMT_BF16;
    } break;
    case IVE_IMAGE_TYPE_U16C1: {
      img_type = CVI_SINGLE;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN) * sizeof(uint16_t);
      strides.push_back(stride);
      heights.push_back(u32Height);
      fmt_size = 2;
      fmt = CVK_FMT_U16;
    } break;
    case IVE_IMAGE_TYPE_S16C1: {
      img_type = CVI_SINGLE;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN) * sizeof(uint16_t);
      strides.push_back(stride);
      heights.push_back(u32Height);
      fmt_size = 2;
      fmt = CVK_FMT_I16;
    } break;
    case IVE_IMAGE_TYPE_U32C1: {
      img_type = CVI_SINGLE;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN) * sizeof(uint32_t);
      strides.push_back(stride);
      heights.push_back(u32Height);
      fmt_size = 4;
      fmt = CVK_FMT_U32;
    } break;
    case IVE_IMAGE_TYPE_FP32C1: {
      img_type = CVI_SINGLE;
      const uint32_t stride = WidthAlign(u32Width, DEFAULT_ALIGN) * sizeof(float);
      strides.push_back(stride);
      heights.push_back(u32Height);
      fmt_size = 4;
      fmt = CVK_FMT_F32;
    } break;
    default:
      LOGE("Not supported enType %s.\n", cviTPUIveImgEnTypeStr[enType]);
      return CVI_FAILURE;
      break;
  }
  if (strides.size() == 0 || heights.size() == 0) {
    LOGE("[DEV] Stride not set.\n");
    return CVI_FAILURE;
  }

  CviImg *buffer_ptr =
      pstBuffer == NULL ? nullptr : reinterpret_cast<CviImg *>(pstBuffer->tpu_block);
  auto *cpp_img = new CviImg(handle_ctx->rt_handle, u32Height, u32Width, strides, heights, img_type,
                             fmt, buffer_ptr);
  if (!cpp_img->IsInit()) {
    LOGE("Failed to init IVE_IMAGE_S.\n");
    return CVI_FAILURE;
  }

  pstImg->tpu_block = reinterpret_cast<CVI_IMG *>(cpp_img);
  pstImg->enType = enType;
  pstImg->u16Width = cpp_img->GetImgWidth();
  pstImg->u16Height = cpp_img->GetImgHeight();
  pstImg->u16Reserved = fmt_size;

  size_t i_limit = cpp_img->GetImgChannel();
  for (size_t i = 0; i < i_limit; i++) {
    pstImg->pu8VirAddr[i] = cpp_img->GetVAddr() + cpp_img->GetImgCOffsets()[i];
    pstImg->u64PhyAddr[i] = cpp_img->GetPAddr() + cpp_img->GetImgCOffsets()[i];
    pstImg->u16Stride[i] = cpp_img->GetImgStrides()[i];
  }

  for (size_t i = i_limit; i < 3; i++) {
    pstImg->pu8VirAddr[i] = NULL;
    pstImg->u64PhyAddr[i] = -1;
    pstImg->u16Stride[i] = 0;
  }
  return CVI_SUCCESS;
}

CVI_S32 TPU_IVE_CreateImage(IVE_HANDLE pIveHandle, IVE_IMAGE_S *pstImg, IVE_IMAGE_TYPE_E enType,
                            CVI_U32 u32Width, CVI_U32 u32Height) {
  return TPU_IVE_CreateImage2(pIveHandle, pstImg, enType, u32Width, u32Height, NULL);
}

CVI_S32 TPU_IVE_SubImage(IVE_HANDLE pIveHandle, IVE_SRC_IMAGE_S *pstSrc, IVE_DST_IMAGE_S *pstDst,
                         CVI_U16 u16X1, CVI_U16 u16Y1, CVI_U16 u16X2, CVI_U16 u16Y2) {
  memset(pstDst, 0, sizeof(IVE_DST_IMAGE_S));
  if (!IsValidImageType(pstSrc, STRFY(pstSrc), IVE_IMAGE_TYPE_U8C1, IVE_IMAGE_TYPE_U8C3_PLANAR,
                        IVE_IMAGE_TYPE_BF16C1, IVE_IMAGE_TYPE_YUV422P, IVE_IMAGE_TYPE_YUV420P,
                        IVE_IMAGE_TYPE_YUV420SP)) {
    LOGE("TPU_IVE_SubImage fail,not valid type\n");
    return CVI_FAILURE;
  }
  if (u16X1 >= u16X2 || u16Y1 >= u16Y2) {
    LOGE("(X1, Y1) must smaller than (X2, Y2).\n");
    return CVI_FAILURE;
  }
  if ((u16X1 % 2 != 0 || u16X2 % 2 != 0) && pstSrc->enType == IVE_IMAGE_TYPE_YUV422P) {
    LOGE("(X1, X2) must all not be odd.\n");
    return CVI_FAILURE;
  }
  if ((u16X1 % 2 != 0 || u16X2 % 2 != 0) && (u16Y1 % 2 != 0 || u16Y2 % 2 != 0) &&
      (pstSrc->enType == IVE_IMAGE_TYPE_YUV420P || pstSrc->enType == IVE_IMAGE_TYPE_YUV420SP)) {
    LOGE("(X1, X2, Y1, Y2) must all not be odd.\n");
    return CVI_FAILURE;
  }
  if (pstDst->tpu_block != NULL) {
    LOGE("pstDst must be empty.\n");
    return CVI_FAILURE;
  }
  IVE_HANDLE_CTX *handle_ctx = reinterpret_cast<IVE_HANDLE_CTX *>(pIveHandle);
  auto *src_img = reinterpret_cast<CviImg *>(pstSrc->tpu_block);
  auto *cpp_img = new CviImg(handle_ctx->rt_handle, *src_img, u16X1, u16Y1, u16X2, u16Y2);
  if (cpp_img->GetVAddr() == nullptr) {
    LOGE("generate sub image failed\n");
    delete cpp_img;
    return CVI_FAILURE;
  }
  pstDst->tpu_block = reinterpret_cast<CVI_IMG *>(cpp_img);

  pstDst->enType = pstSrc->enType;
  pstDst->u16Width = cpp_img->m_tg.shape.w;
  pstDst->u16Height = cpp_img->m_tg.shape.h;
  pstDst->u16Reserved = pstSrc->u16Reserved;

  size_t num_plane = cpp_img->GetImgCOffsets().size() - 1;
  // LOGD("src width:%d,src
  // stride0:%d,stride1:%d\n",pstSrc->u16Width,pstSrc->u16Stride[0],pstSrc->u16Stride[1]);
  // LOGD("channel:%d,numplane:%d\n", (int)cpp_img->m_tg.shape.c, (int)num_plane);
  for (size_t i = 0; i < num_plane; i++) {
    pstDst->pu8VirAddr[i] = cpp_img->GetVAddr() + cpp_img->GetImgCOffsets()[i];
    pstDst->u64PhyAddr[i] = cpp_img->GetPAddr() + cpp_img->GetImgCOffsets()[i];
    pstDst->u16Stride[i] = cpp_img->GetImgStrides()[i];
    // LOGD("updatesubimg ,plane:%d,coffset:%d,stride:%d\n", (int)i,
    // (int)cpp_img->GetImgCOffsets()[i],pstDst->u16Stride[i]);
  }

  // LOGD("subimg planeoffset:%d,%d,%d\n", (int)(pstDst->u64PhyAddr[1] - pstSrc->u64PhyAddr[0]),
  //      (int)(pstDst->u64PhyAddr[2] - pstSrc->u64PhyAddr[1]),
  //      (int)(pstDst->u64PhyAddr[3] - pstSrc->u64PhyAddr[2]));

  for (size_t i = num_plane; i < 3; i++) {
    pstDst->pu8VirAddr[i] = NULL;
    pstDst->u64PhyAddr[i] = 0;
    pstDst->u16Stride[i] = 0;
  }
  return CVI_SUCCESS;
}

CVI_S32 TPU_IVE_DMA(IVE_HANDLE pIveHandle, IVE_IMAGE_S *pstSrc, IVE_IMAGE_S *pstDst,
                    IVE_DMA_CTRL_S *pstDmaCtrl, bool bInstant) {
  ScopedTrace t(__PRETTY_FUNCTION__);
  if (CVI_IVE_ImageInit(pstSrc) != CVI_SUCCESS) {
    LOGE("Source cannot be inited.\n");
    return CVI_FAILURE;
  }
  if (CVI_IVE_ImageInit(pstDst) != CVI_SUCCESS) {
    LOGE("Destination cannot be inited.\n");
    return CVI_FAILURE;
  }
  // Special check with YUV 420 and 422, for copy only
  if ((pstSrc->enType == IVE_IMAGE_TYPE_YUV420P || pstSrc->enType == IVE_IMAGE_TYPE_YUV420SP ||
       pstSrc->enType == IVE_IMAGE_TYPE_YUV422P) &&
      pstDmaCtrl->enMode != IVE_DMA_MODE_DIRECT_COPY) {
    LOGE("Currently only supports IVE_DMA_MODE_DIRECT_COPY for YUV 420 and 422 images.");
    return CVI_FAILURE;
  }
  int ret = CVI_FAILURE;
  IVE_HANDLE_CTX *handle_ctx = reinterpret_cast<IVE_HANDLE_CTX *>(pIveHandle);
  if (pstDmaCtrl->enMode == IVE_DMA_MODE_DIRECT_COPY) {
#ifdef USE_CPU_COPY
    CVI_IVE_BufRequest(pIveHandle, pstSrc);
    CVI_IVE_BufRequest(pIveHandle, pstDst);
    uint size = pstSrc->u16Stride[0] * pstSrc->u32Height;
    memcpy(pstDst->pu8VirAddr[0], pstSrc->pu8VirAddr[0], size);
    CVI_IVE_BufFlush(pIveHandle, pstSrc);
    CVI_IVE_BufFlush(pIveHandle, pstDst);
#else
    CviImg *cpp_src = reinterpret_cast<CviImg *>(pstSrc->tpu_block);
    CviImg *cpp_dst = reinterpret_cast<CviImg *>(pstDst->tpu_block);
    // std::vector<CviImg*> inputs = {cpp_src};
    // std::vector<CviImg*> outputs = {cpp_dst};
    // LOGD("use IVE_DMA_MODE_DIRECT_COPY\n");
    ret = IveTPUCopyDirect::run(handle_ctx->rt_handle, handle_ctx->cvk_ctx, cpp_src, cpp_dst);
#endif
  } else if (pstDmaCtrl->enMode == IVE_DMA_MODE_INTERVAL_COPY) {
    handle_ctx->t_h.t_copy_int.setInvertal(pstDmaCtrl->u8HorSegSize, pstDmaCtrl->u8VerSegRows);
    handle_ctx->t_h.t_copy_int.init(handle_ctx->rt_handle, handle_ctx->cvk_ctx);
    CviImg *cpp_src = reinterpret_cast<CviImg *>(pstSrc->tpu_block);
    CviImg *cpp_dst = reinterpret_cast<CviImg *>(pstDst->tpu_block);
    std::vector<CviImg *> inputs = {cpp_src};
    std::vector<CviImg *> outputs = {cpp_dst};

    ret = handle_ctx->t_h.t_copy_int.run(handle_ctx->rt_handle, handle_ctx->cvk_ctx, inputs,
                                         outputs, true);
  }
  return ret;
}

CVI_S32 TPU_SYS_FreeI(IVE_HANDLE pIveHandle, IVE_IMAGE_S *pstImg) {
  if (pstImg->tpu_block == NULL) {
    LOGD("Image tpu block is freed.\n");
    return CVI_SUCCESS;
  }
  auto *cpp_img = reinterpret_cast<CviImg *>(pstImg->tpu_block);
  if (!cpp_img->IsNullMem()) {
    if (pIveHandle == NULL) {
      LOGE("should have ive handle to release cpp_img inside memory");
    }
    IVE_HANDLE_CTX *handle_ctx = reinterpret_cast<IVE_HANDLE_CTX *>(pIveHandle);
    cpp_img->Free(handle_ctx->rt_handle);
  }
  delete cpp_img;
  pstImg->tpu_block = NULL;
  return CVI_SUCCESS;
}

CVI_S32 TPU_IVE_Blend_ROI_Impl(IVE_HANDLE pIveHandle, VIDEO_FRAME_INFO_S *pstSrc1,
                               VIDEO_FRAME_INFO_S *pstSrc2, VIDEO_FRAME_INFO_S *pstAlphaY,
                               VIDEO_FRAME_INFO_S *pstAlphaUV, uint32_t roi_w,
                               VIDEO_FRAME_INFO_S *pstDst) {
  VIDEO_FRAME_S *pstVFSrc = &pstSrc1->stVFrame;
  uint32_t width = pstVFSrc->u32Width;
  uint32_t height = pstVFSrc->u32Height;

  int ret = CVI_SUCCESS;

  VIDEO_FRAME_INFO_S *weights[2] = {pstAlphaY, pstAlphaUV};
  for (int i = 0; i < 2; i++) {  // process Y and UV plane seperately
    // struct timeval t_VF2I_1, t_VF2I_2;
    // gettimeofday(&t_VF2I_1, NULL);
    IVE_IMAGE_S planei_src1, planei_src2, planei_dst;
    ret = VideoFrameYInfo2Image(pstSrc1, &planei_src1, i);
    if (ret != CVI_SUCCESS) {
      LOGE("VideoFrameYInfo2Image faild\n");
      return CVI_FAILURE;
    }

    ret = VideoFrameYInfo2Image(pstSrc2, &planei_src2, i);
    if (ret != CVI_SUCCESS) {
      LOGE("VideoFrameYInfo2Image faild\n");
      return CVI_FAILURE;
    }

    ret = VideoFrameYInfo2Image(pstDst, &planei_dst, i);
    if (ret != CVI_SUCCESS) {
      LOGE("VideoFrameYInfo2Image faild\n");
      return CVI_FAILURE;
    }
    // gettimeofday(&t_VF2I_2, NULL);
    // unsigned long elapsed_cpu1 = (t_VF2I_2.tv_sec - t_VF2I_1.tv_sec) * 1000000 + t_VF2I_2.tv_usec
    // - t_VF2I_1.tv_usec; printf("VideoFrameInfo to image time, elapsed: %lu us\n", elapsed_cpu1);

    // struct timeval t_subI_0, t_subI_1;
    // gettimeofday(&t_subI_0, NULL);
    IVE_IMAGE_S planei_sub1, planei_sub2, planei_sub_dst, alpha;
    ret = VideoFrameYInfo2Image(weights[i], &alpha, 0);
    ret = TPU_IVE_SubImage(pIveHandle, &planei_src1, &planei_sub1, width - roi_w, 0, width,
                           height / (i + 1));
    ret = TPU_IVE_SubImage(pIveHandle, &planei_src2, &planei_sub2, 0, 0, roi_w, height / (i + 1));
    ret = TPU_IVE_SubImage(pIveHandle, &planei_dst, &planei_sub_dst, width - roi_w, 0, width,
                           height / (i + 1));
    // gettimeofday(&t_subI_1, NULL);
    // unsigned long elapsed_cpu2 = (t_subI_1.tv_sec - t_subI_0.tv_sec) * 1000000 + t_subI_1.tv_usec
    // - t_subI_0.tv_usec; printf("SubImage time, elapsed: %lu us\n", elapsed_cpu2);

    /**/
    // subImage copy
    ///////////////////////////////////////////////////
    if (planei_sub1.u16Width != g_tmp_sub_img[2 * i + 0].u16Width ||
        planei_sub1.u16Height != g_tmp_sub_img[2 * i + 0].u16Height) {
      // std::cout << "create new image1\n";
      TPU_SYS_FreeI(pIveHandle, &g_tmp_sub_img[2 * i + 0]);
      TPU_IVE_CreateImage(pIveHandle, &g_tmp_sub_img[2 * i + 0], IVE_IMAGE_TYPE_U8C1,
                          planei_sub1.u16Width, planei_sub1.u16Height);
    }
    if (planei_sub2.u16Width != g_tmp_sub_img[2 * i + 1].u16Width ||
        planei_sub2.u16Height != g_tmp_sub_img[2 * i + 1].u16Height) {
      // std::cout << "create new image2\n";
      TPU_SYS_FreeI(pIveHandle, &g_tmp_sub_img[2 * i + 1]);
      TPU_IVE_CreateImage(pIveHandle, &g_tmp_sub_img[2 * i + 1], IVE_IMAGE_TYPE_U8C1,
                          planei_sub1.u16Width, planei_sub1.u16Height);
    }

    // IVE_IMAGE_S planei_sub1_new, planei_sub2_new;
    // // IVE_IMAGE_S planei_sub_dst_new;
    // /////////////////////////////////////////////////////
    // TPU_IVE_CreateImage(pIveHandle, &planei_sub1_new, IVE_IMAGE_TYPE_U8C1, planei_sub1.u16Width,
    // planei_sub1.u16Height); TPU_IVE_CreateImage(pIveHandle, &planei_sub2_new,
    // IVE_IMAGE_TYPE_U8C1, planei_sub2.u16Width, planei_sub2.u16Height);
    // TPU_IVE_CreateImage(pIveHandle, &planei_sub_dst_new, IVE_IMAGE_TYPE_U8C1,
    // planei_sub_dst.u16Width, planei_sub_dst.u16Height);

    // printf("Run TPU direct copy.\n");

    // struct timeval t0, t1;
    // struct timeval t2, t3;
    // struct timeval t4, t5;
    // gettimeofday(&t0, NULL);
    IVE_DMA_CTRL_S iveDmaCtrl;
    iveDmaCtrl.enMode = IVE_DMA_MODE_DIRECT_COPY;
    TPU_IVE_DMA(pIveHandle, &planei_sub1, &g_tmp_sub_img[2 * i + 0], &iveDmaCtrl, 0);
    TPU_IVE_DMA(pIveHandle, &planei_sub2, &g_tmp_sub_img[2 * i + 1], &iveDmaCtrl, 0);
    // TPU_IVE_DMA(pIveHandle, &planei_sub1, &planei_sub1_new, &iveDmaCtrl, 0);
    // TPU_IVE_DMA(pIveHandle, &planei_sub2, &planei_sub2_new, &iveDmaCtrl, 0);
    // gettimeofday(&t1, NULL);
    // unsigned long elapsed_cpu = (t1.tv_sec - t0.tv_sec) * 1000000 + t1.tv_usec - t0.tv_usec;
    // printf("Run width of roi_w DMA done, elapsed: %lu us, ret = %s.\n", elapsed_cpu,
    //         ret == CVI_SUCCESS ? "Success" : "Fail");

    // LOGD("alpha,stride_w:%u,width:%u\n",alpha.u16Stride[0],alpha.u16Width);
    // gettimeofday(&t2, NULL);
    // ret = CVI_IVE_Blend_Pixel(pIveHandle, &planei_sub1_new, &planei_sub2_new, &alpha,
    // &planei_sub_dst_new, false); TPU_IVE_DMA(pIveHandle, &planei_sub_dst_new, &planei_sub_dst,
    // &iveDmaCtrl, 0); gettimeofday(&t3, NULL); elapsed_cpu = (t3.tv_sec - t2.tv_sec) * 1000000 +
    // t3.tv_usec - t2.tv_usec; printf("Run Blend + DMA done, elapsed: %lu us, ret = %s.\n",
    // elapsed_cpu,
    //         ret == CVI_SUCCESS ? "Success" : "Fail");

    // gettimeofday(&t4, NULL);
    // ret = CVI_IVE_Blend_Pixel(pIveHandle, &planei_sub1_new, &planei_sub2_new, &alpha,
    // &planei_sub_dst, false);
    ret = CVI_IVE_Blend_Pixel(pIveHandle, &g_tmp_sub_img[2 * i + 0], &g_tmp_sub_img[2 * i + 1],
                              &alpha, &planei_sub_dst, false);
    // gettimeofday(&t5, NULL);
    // elapsed_cpu = (t5.tv_sec - t4.tv_sec) * 1000000 + t5.tv_usec - t4.tv_usec;
    // printf("Run Blend directly to subimage of result done, elapsed: %lu us, ret = %s.\n",
    // elapsed_cpu,
    //         ret == CVI_SUCCESS ? "Success" : "Fail");

    // ret = CVI_IVE_Blend_Pixel(pIveHandle, &planei_sub1, &planei_sub2, &alpha, &planei_sub_dst,
    // false);
    if (ret != CVI_SUCCESS) {
      LOGE("CVI_IVE_Blend_Pixel faild\n");
      return ret;
    }
    TPU_Free_Image(&planei_src1);
    TPU_Free_Image(&planei_src2);
    TPU_Free_Image(&planei_dst);

    TPU_Free_Image(&planei_sub1);
    TPU_Free_Image(&planei_sub2);
    // TPU_SYS_FreeI(pIveHandle, &planei_sub1_new);
    // TPU_SYS_FreeI(pIveHandle, &planei_sub2_new);
    // TPU_SYS_FreeI(pIveHandle, &planei_sub_dst_new);
    TPU_Free_Image(&planei_sub_dst);
    TPU_Free_Image(&alpha);
  }
  return ret;
}

CVI_S32 CVI_IVE_TPU_Blend_ROI(IVE_HANDLE pIveHandle, VIDEO_FRAME_INFO_S *pstSrc1,
                              VIDEO_FRAME_INFO_S *pstSrc2, VIDEO_FRAME_INFO_S *pstAlphaY,
                              VIDEO_FRAME_INFO_S *pstAlphaUV, uint32_t roi_w,
                              VIDEO_FRAME_INFO_S *pstDst) {
  bool src1_type = pstSrc1->stVFrame.enPixelFormat == PIXEL_FORMAT_NV12 ||
                   pstSrc1->stVFrame.enPixelFormat == PIXEL_FORMAT_NV21;
  bool src2_type = pstSrc2->stVFrame.enPixelFormat == PIXEL_FORMAT_NV12 ||
                   pstSrc2->stVFrame.enPixelFormat == PIXEL_FORMAT_NV21;
  bool dst_type = pstDst->stVFrame.enPixelFormat == PIXEL_FORMAT_NV12 ||
                  pstDst->stVFrame.enPixelFormat == PIXEL_FORMAT_NV21;

  if (!src1_type || !src2_type || !dst_type) {
    LOGE("pstSrc1,pstSrc2,pstDst only support PIXEL_FORMAT_NV12 or PIXEL_FORMAT_NV21 format\n");
    return CVI_FAILURE;
  }

  IVE_IMAGE_S src1, src2, dst;
  CVI_S32 ret = CVI_SUCCESS;

  ret = TPU_IVE_VideoFrameInfo2Image(pstSrc1, &src1);
  if (ret != CVI_SUCCESS) {
    LOGE("Convert pstSrc1 to IVE_IMAGE_S Failed!");
    return ret;
  }

  ret = TPU_IVE_VideoFrameInfo2Image(pstSrc2, &src2);
  if (ret != CVI_SUCCESS) {
    LOGE("Convert pstSrc2 to IVE_IMAGE_S Failed!");
    return ret;
  }

  ret = TPU_IVE_VideoFrameInfo2Image(pstDst, &dst);
  if (ret != CVI_SUCCESS) {
    LOGE("Convert pstDst to IVE_IMAGE_S Failed!");
    return ret;
  }

  int width = src1.u16Width;
  int height = src1.u16Height;
  int outw = 2 * width - roi_w;

  if (src1.u16Width != src2.u16Width || src1.u16Height != src2.u16Height) {
    LOGE("pstSrc1 and pstSrc2 size not equal\n");
    return CVI_FAILURE;
  }

  if (pstAlphaY->stVFrame.u32Width != roi_w || dst.u16Width != src2.u16Width * 2 - roi_w) {
    LOGE("error,pstAlphaY.width(%d) != roi_w(%d) || dst.width(%d)!= src2.height*2-roi_w(%d)\n",
         pstAlphaY->stVFrame.u32Width, roi_w, dst.u16Width, src2.u16Width * 2 - roi_w);
    return CVI_FAILURE;
  }
  if (pstAlphaUV->stVFrame.u32Width != roi_w ||
      pstAlphaUV->stVFrame.u32Height != src1.u16Height / 2) {
    LOGE("error,pstAlphaUV.width(%d) != roi_w(%d) || pstAlphaUV.height(%d)!= src1.height/2(%d)\n",
         pstAlphaUV->stVFrame.u32Width, roi_w, pstAlphaUV->stVFrame.u32Height, height / 2);
    return CVI_FAILURE;
  }

  IVE_IMAGE_S sub11, sub22, sub1, sub3;

  // LOGD("src1_addr0:%0x,src1_addr1:%0x,stride0:%u,stride1:%u,\n",pstSrc1->stVFrame.u64PhyAddr[0],pstSrc1->stVFrame.u64PhyAddr[1],pstSrc1->stVFrame.u16Stride[0],pstSrc1->stVFrame.u16Stride[1]);
  // LOGD("src2_addr0:%0x,src2_addr1:%0x,stride0:%u,stride1:%u,\n",pstSrc2->stVFrame.u64PhyAddr[0],pstSrc2->stVFrame.u64PhyAddr[1],pstSrc2->stVFrame.u16Stride[0],pstSrc2->stVFrame.u16Stride[1]);
  // LOGD("dst_addr0:%0x,dst_addr1:%0x,stride0:%u,stride1:%u,\n",pstDst->stVFrame.u64PhyAddr[0],pstDst->stVFrame.u64PhyAddr[1],pstDst->stVFrame.u16Stride[0],pstDst->stVFrame.u16Stride[1]);

  TPU_IVE_SubImage(pIveHandle, &dst, &sub1, 0, 0, width - roi_w, height);
  TPU_IVE_SubImage(pIveHandle, &dst, &sub3, width, 0, outw, height);
  TPU_IVE_SubImage(pIveHandle, &src1, &sub11, 0, 0, width - roi_w, height);
  TPU_IVE_SubImage(pIveHandle, &src2, &sub22, roi_w, 0, width, height);

  // LOGD("Run TPU Direct Copy.\n");
  IVE_DMA_CTRL_S iveDmaCtrl;
  iveDmaCtrl.enMode = IVE_DMA_MODE_DIRECT_COPY;
  //////////////////////////////////////////////////
  // struct timeval t0, t1, t2, t3;
  // gettimeofday(&t0, NULL);
  ret = TPU_IVE_DMA(pIveHandle, &sub11, &sub1, &iveDmaCtrl, 0);
  if (ret != CVI_SUCCESS) {
    LOGE("TPU_IVE_DMA faild,sub11 to sub1\n");
  }
  ret = TPU_IVE_DMA(pIveHandle, &sub22, &sub3, &iveDmaCtrl, 0);
  if (ret != CVI_SUCCESS) {
    LOGE("TPU_IVE_DMA faild,sub22 to sub3\n");
  }
  // gettimeofday(&t1, NULL);
  // unsigned long elapsed_cpu = (t1.tv_sec - t0.tv_sec) * 1000000 + t1.tv_usec - t0.tv_usec;
  // printf("Run IVE DMA done, elapsed: %lu us, ret = %s.\n", elapsed_cpu,
  //         ret == CVI_SUCCESS ? "Success" : "Fail");

  // LOGD("to do blend\n");

  // gettimeofday(&t2, NULL);
  ret = TPU_IVE_Blend_ROI_Impl(pIveHandle, pstSrc1, pstSrc2, pstAlphaY, pstAlphaUV, roi_w, pstDst);
  // gettimeofday(&t3, NULL);
  // elapsed_cpu = (t3.tv_sec - t2.tv_sec) * 1000000 + t3.tv_usec - t2.tv_usec;
  // printf("Run IVE alpha blending done, elapsed: %lu us, ret = %s.\n", elapsed_cpu,
  //         ret == CVI_SUCCESS ? "Success" : "Fail");

  // LOGD("to free\n");
  TPU_Free_Image(&src1);
  TPU_Free_Image(&src2);
  TPU_Free_Image(&dst);

  TPU_Free_Image(&sub11);
  TPU_Free_Image(&sub22);
  TPU_Free_Image(&sub1);
  TPU_Free_Image(&sub3);

  return ret;
}
