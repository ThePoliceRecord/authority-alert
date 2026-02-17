#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/time.h>
#include "cvi_tpu_ive.h"
#include "ive.h"

void writeYUV420SP(IVE_HANDLE handle, const char *filename, const uint16_t width,
                   const uint16_t height, IVE_DST_IMAGE_S *img) {
  CVI_IVE_BufRequest(handle, img);
  FILE *yuv = fopen(filename, "wb");
  for (int i = 0; i < height; i++) {
    fwrite(img->pu8VirAddr[0] + i * img->u16Stride[0], width, sizeof(uint8_t), yuv);
  }
  uint16_t height_2 = height / 2;
  for (int i = 0; i < height_2; i++) {
    fwrite(img->pu8VirAddr[1] + i * img->u16Stride[1], width, sizeof(uint8_t), yuv);
  }
  fclose(yuv);
}

int check_roi_blend_eq(IVE_HANDLE handle, IVE_DST_IMAGE_S *p_src1, IVE_DST_IMAGE_S *p_src2,
                       IVE_DST_IMAGE_S *p_alpha, IVE_DST_IMAGE_S *p_beta, IVE_DST_IMAGE_S *p_dst,
                       int roi_w) {
  uint32_t width = p_src1->u16Width;
  uint32_t height = p_src1->u16Height;
  CVI_IVE_BufRequest(handle, p_dst);

  for (int r = 0; r < height; r++) {
    uint8_t *ptr1 = p_src1->pu8VirAddr[0] + r * p_src1->u16Stride[0] + width - roi_w;
    uint8_t *ptr2 = p_src2->pu8VirAddr[0] + r * p_src2->u16Stride[0];
    uint8_t *ptr_dst = p_dst->pu8VirAddr[0] + r * p_dst->u16Stride[0] + width - roi_w;

    uint8_t *ptr_alpha = p_alpha->pu8VirAddr[0] + r * p_alpha->u16Stride[0];

    for (int c = 0; c < roi_w; c++) {
      int src_val = ptr1[c] * ptr_alpha[c] + ptr2[c] * (255 - ptr_alpha[c]);
      int retval = src_val / 256.0 + 0.5;
      if (retval > 255) retval = 255;
      if (retval != ptr_dst[c]) {
        printf("fail,[r:%d,c:%d] u8_a:%d,u8_b:%d,u8_wa:%u,TPU %d, CPU %d,srcval:%d\n", r, c,
               ptr1[c], ptr2[c], ptr_alpha[c], ptr_dst[c], retval, src_val);
        //   printBits(2, &src_val);
        printf("\n");
        return CVI_FAILURE;
      } else if (c == 0 || c == roi_w - 1) {
        printf("ok,[r:%d,c:%d] u8_a:%d,u8_b:%d,u8_wa:%u,TPU %d, CPU %d,srcval:%d\n", r, c, ptr1[c],
               ptr2[c], ptr_alpha[c], ptr_dst[c], retval, src_val);
      }
    }
  }

  for (int r = 0; r < height / 2; r++) {
    uint8_t *ptr1 = p_src1->pu8VirAddr[1] + r * p_src1->u16Stride[1] + width - roi_w;
    uint8_t *ptr2 = p_src2->pu8VirAddr[1] + r * p_src2->u16Stride[1];
    uint8_t *ptr_dst = p_dst->pu8VirAddr[1] + r * p_dst->u16Stride[1] + width - roi_w;

    uint8_t *ptr_alpha = p_beta->pu8VirAddr[0] + r * p_beta->u16Stride[0];

    for (int c = 0; c < roi_w; c++) {
      int src_val = ptr1[c] * ptr_alpha[c] + ptr2[c] * (255 - ptr_alpha[c]);
      int retval = src_val / 256.0 + 0.5;
      if (retval > 255) retval = 255;
      if (retval != ptr_dst[c]) {
        printf("fail,[r:%d,c:%d] u8_a:%d,u8_b:%d,u8_wa:%u,TPU %d, CPU %d,srcval:%d\n", r, c,
               ptr1[c], ptr2[c], ptr_alpha[c], ptr_dst[c], retval, src_val);
        //   printBits(2, &src_val);
        printf("\n");
        return CVI_FAILURE;
      } else if (c == 0 || c == roi_w - 1) {
        printf("ok,[r:%d,c:%d] u8_a:%d,u8_b:%d,u8_wa:%u,TPU %d, CPU %d,srcval:%d\n", r, c, ptr1[c],
               ptr2[c], ptr_alpha[c], ptr_dst[c], retval, src_val);
      }
    }
  }
  printf("check eq\n");
  return 0;
}

int main(int argc, char **argv) {
  // if (argc != 3) {
  //   printf("Incorrect parameter. Usage: %s <image1> <image2>\n", argv[0]);
  //   return CVI_FAILURE;
  // }

  const char *file_name1 = argv[1];
  const char *file_name2 = argv[2];
  const char *file_wy = argv[3];
  const char *file_wuv = argv[4];
  const char *dst_blend = argv[5];

  // Create instance
  printf("Create instance.\n");
  IVE_HANDLE handle = CVI_TPU_IVE_CreateHandle();
  IVE_IMAGE_S src1, src2, weight_y, weight_uv;
  uint32_t width = 2560;
  uint32_t height = 1440;
  uint32_t roi_w = 32;
  int ret;
  ret = CVI_IVE_ReadRawImage(handle, &src1, file_name1, IVE_IMAGE_TYPE_YUV420SP, width, height);
  ret = CVI_IVE_ReadRawImage(handle, &src2, file_name2, IVE_IMAGE_TYPE_YUV420SP, width, height);

  ret = CVI_IVE_ReadRawImage(handle, &weight_y, file_wy, IVE_IMAGE_TYPE_U8C1, roi_w, height);
  ret = CVI_IVE_ReadRawImage(handle, &weight_uv, file_wuv, IVE_IMAGE_TYPE_U8C1, roi_w, height / 2);

  printf("Convert image from RGB to YUV420P\n");
  IVE_IMAGE_S dst_nv12;
  int outw = 2 * width - roi_w;
  CVI_IVE_CreateImage(handle, &dst_nv12, IVE_IMAGE_TYPE_YUV420SP, outw, height);
  memset(dst_nv12.pu8VirAddr[0], 0, dst_nv12.u16Height * dst_nv12.u16Stride[0]);
  memset(dst_nv12.pu8VirAddr[1], 0, dst_nv12.u16Height / 2 * dst_nv12.u16Stride[1]);

  // IVE_DMA_CTRL_S iveDmaCtrl;
  // iveDmaCtrl.enMode = IVE_DMA_MODE_DIRECT_COPY;

  // IVE_DST_IMAGE_S dst2_1, dst2_2, src_1, src_2;
  // memset(&dst2_1, 0, sizeof(IVE_DST_IMAGE_S));
  // memset(&dst2_2, 0, sizeof(IVE_DST_IMAGE_S));
  // memset(&src_1, 0, sizeof(IVE_DST_IMAGE_S));
  // memset(&src_2, 0, sizeof(IVE_DST_IMAGE_S));

  // CVI_IVE_SubImage(handle, &dst_nv12, &dst2_1, 0, 0, width-roi_w, height);
  // CVI_IVE_SubImage(handle, &dst_nv12, &dst2_2, width, 0, outw, height);
  // CVI_IVE_SubImage(handle, &src1, &src_1, 0, 0, width-roi_w, height);
  // CVI_IVE_SubImage(handle, &src2, &src_2,roi_w, 0, width, height);
  // ret |= CVI_IVE_DMA(handle, &src_1, &dst2_1, &iveDmaCtrl, 0);
  // ret |= CVI_IVE_DMA(handle, &src_2, &dst2_2, &iveDmaCtrl, 0);

  printf("Run IVE alpha blending.\n");
  // Run IVE blend.

  VIDEO_FRAME_INFO_S vf_src1, vf_src2, vf_y, vf_uv, vf_dst;
  ret = CVI_IVE_Image2VideoFrameInfo(&src1, &vf_src1);
  ret = CVI_IVE_Image2VideoFrameInfo(&src2, &vf_src2);
  ret = CVI_IVE_Image2VideoFrameInfo(&weight_y, &vf_y);
  ret = CVI_IVE_Image2VideoFrameInfo(&weight_uv, &vf_uv);
  ret = CVI_IVE_Image2VideoFrameInfo(&dst_nv12, &vf_dst);

  struct timeval t0, t1;
  gettimeofday(&t0, NULL);
  ret = CVI_IVE_TPU_Blend_ROI(handle, &vf_src1, &vf_src2, &vf_y, &vf_uv, roi_w, &vf_dst);
  gettimeofday(&t1, NULL);
  unsigned long elapsed_cpu = (t1.tv_sec - t0.tv_sec) * 1000000 + t1.tv_usec - t0.tv_usec;
  printf("Run IVE alpha blending done, elapsed: %lu us, ret = %s.\n", elapsed_cpu,
         ret == CVI_SUCCESS ? "Success" : "Fail");

  check_roi_blend_eq(handle, &src1, &src2, &weight_y, &weight_uv, &dst_nv12, roi_w);
  writeYUV420SP(handle, dst_blend, outw, height, &dst_nv12);
  // CVI_IVE_BufRequest(handle, &dst_nv12);
  // FILE *fp = fopen(dst_blend,"wb");
  // int outh = (int)(height*1.5);
  // uint8_t *ptr_row = dst_nv12.pu8VirAddr[0];
  // int stride = dst_nv12.u16Stride[0];
  // for(int i = 0; i < outh;i++){
  //   ptr_row = ptr_row + stride;
  //   fwrite(ptr_row,outw,1,fp);
  // }
  // // printf("to write size:%d\n",outlen);
  // // fwrite(dst_nv12.pu8VirAddr[0],outlen,1,fp);
  // fclose(fp);

  // Free memory, instance
  CVI_SYS_FreeI(handle, &weight_y);
  CVI_SYS_FreeI(handle, &src1);
  CVI_SYS_FreeI(handle, &src2);
  CVI_SYS_FreeI(handle, &weight_uv);
  CVI_SYS_FreeI(handle, &dst_nv12);
  // CVI_SYS_FreeI(handle, &dst);
  CVI_TPU_IVE_DestroyHandle(handle);
  return ret;
}
