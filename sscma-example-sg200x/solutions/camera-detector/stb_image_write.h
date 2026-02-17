/* stb_image_write - v1.16 - public domain - http://nothings.org/stb
   writes out PNG/BMP/TGA/JPEG/HDR images to C stdio - Sean Barrett 2010-2015
                                     no warranty implied; use at your own risk

   Before #including,

       #define STB_IMAGE_WRITE_IMPLEMENTATION

   in the file that you want to have the implementation.

   Will probably not work correctly with potential exceptions, but as the
   original specification of C89 did not have exceptions, I'm not going to
   worry about it.

LICENSE
  See end of file for license information.
*/

#ifndef INCLUDE_STB_IMAGE_WRITE_H
#define INCLUDE_STB_IMAGE_WRITE_H

#include <stdlib.h>

// if STB_IMAGE_WRITE_STATIC causes problems, try defining STBIWDEF to 'inline' or 'static inline'
#ifndef STBIWDEF
#ifdef STB_IMAGE_WRITE_STATIC
#define STBIWDEF  static
#else
#define STBIWDEF  extern
#endif
#endif

#ifndef STB_IMAGE_WRITE_STATIC  // C++ forbids static forward declarations
STBIWDEF int stbi_write_jpg_to_func(void (*func)(void *context, void *data, int size), void *context, int x, int y, int comp, const void *data, int quality);
STBIWDEF int stbi_write_jpg(char const *filename, int x, int y, int comp, const void *data, int quality);
#endif

typedef void stbi_write_func(void *context, void *data, int size);
STBIWDEF int stbi_write_jpg_to_func(stbi_write_func *func, void *context, int x, int y, int comp, const void *data, int quality);

#endif//INCLUDE_STB_IMAGE_WRITE_H

#ifdef STB_IMAGE_WRITE_IMPLEMENTATION

#ifndef STBIW_ASSERT
#include <assert.h>
#define STBIW_ASSERT(x) assert(x)
#endif

#define STBIW_UCHAR(x) (unsigned char) ((x) & 0xff)

#ifdef STB_IMAGE_WRITE_STATIC
static int stbi__abs(int a) { return a < 0 ? -a : a; }
#else
static int stbi__abs(int a) { return a < 0 ? -a : a; }
#endif

// JPEG encoding - simplified implementation

static const unsigned char stbiw__jpg_ZigZag[] = { 0,1,5,6,14,15,27,28,2,4,7,13,16,26,29,42,3,8,12,17,25,30,41,43,9,11,18,
   24,31,40,44,53,10,19,23,32,39,45,52,54,20,22,33,38,46,51,55,60,21,34,37,47,50,56,59,61,35,36,48,49,57,58,62,63 };

static void stbiw__jpg_writeBits(unsigned char **bitBuf, int *bitCnt, const unsigned short *bs) {
   int value = bs[0];
   int count = bs[1];
   *bitCnt += count;
   *bitBuf += *bitCnt >> 3;
   *bitCnt &= 7;
   while (count > 0) {
      **bitBuf |= (value >> (count - 8 + *bitCnt)) & 0xFF;
      count -= 8 - *bitCnt;
      if (count > 0) {
         (*bitBuf)++;
         *bitCnt = 0;
      }
   }
}

static void stbiw__jpg_DCT(float *d0p, float *d1p, float *d2p, float *d3p, float *d4p, float *d5p, float *d6p, float *d7p) {
   float d0 = *d0p, d1 = *d1p, d2 = *d2p, d3 = *d3p, d4 = *d4p, d5 = *d5p, d6 = *d6p, d7 = *d7p;
   float z1, z2, z3, z4, z5, z11, z13;

   float tmp0 = d0 + d7;
   float tmp7 = d0 - d7;
   float tmp1 = d1 + d6;
   float tmp6 = d1 - d6;
   float tmp2 = d2 + d5;
   float tmp5 = d2 - d5;
   float tmp3 = d3 + d4;
   float tmp4 = d3 - d4;

   // Even part
   float tmp10 = tmp0 + tmp3;
   float tmp13 = tmp0 - tmp3;
   float tmp11 = tmp1 + tmp2;
   float tmp12 = tmp1 - tmp2;

   d0 = tmp10 + tmp11;
   d4 = tmp10 - tmp11;

   z1 = (tmp12 + tmp13) * 0.707106781f;
   d2 = tmp13 + z1;
   d6 = tmp13 - z1;

   // Odd part
   tmp10 = tmp4 + tmp5;
   tmp11 = tmp5 + tmp6;
   tmp12 = tmp6 + tmp7;

   z5 = (tmp10 - tmp12) * 0.382683433f;
   z2 = tmp10 * 0.541196100f + z5;
   z4 = tmp12 * 1.306562965f + z5;
   z3 = tmp11 * 0.707106781f;

   z11 = tmp7 + z3;
   z13 = tmp7 - z3;

   *d5p = z13 + z2;
   *d3p = z13 - z2;
   *d1p = z11 + z4;
   *d7p = z11 - z4;

   *d0p = d0;  *d2p = d2;  *d4p = d4;  *d6p = d6;
}

static void stbiw__jpg_calcBits(int val, unsigned short bits[2]) {
   int tmp1 = val < 0 ? -val : val;
   val = val < 0 ? val-1 : val;
   bits[1] = 1;
   while(tmp1 >>= 1) ++bits[1];
   bits[0] = val & ((1<<bits[1])-1);
}

static int stbiw__jpg_processDU(unsigned char **bitBuf, int *bitCnt, float *CDU, int du_stride, float *fdtbl, int DC, const unsigned short HTDC[256][2], const unsigned short HTAC[256][2]) {
   const unsigned short EOB[2] = { HTAC[0x00][0], HTAC[0x00][1] };
   const unsigned short M16zeroes[2] = { HTAC[0xF0][0], HTAC[0xF0][1] };
   int dataOff, i, j, n, diff, end0pos, x, y;
   int DU[64];

   // DCT rows
   for(dataOff=0; dataOff<64; dataOff+=8) {
      stbiw__jpg_DCT(&CDU[dataOff], &CDU[dataOff+1], &CDU[dataOff+2], &CDU[dataOff+3], &CDU[dataOff+4], &CDU[dataOff+5], &CDU[dataOff+6], &CDU[dataOff+7]);
   }
   // DCT columns
   for(dataOff=0; dataOff<8; ++dataOff) {
      stbiw__jpg_DCT(&CDU[dataOff], &CDU[dataOff+8], &CDU[dataOff+16], &CDU[dataOff+24], &CDU[dataOff+32], &CDU[dataOff+40], &CDU[dataOff+48], &CDU[dataOff+56]);
   }
   // Quantize/descale/zigzag the coefficients
   for(i=0; i<64; ++i) {
      float v = CDU[i]*fdtbl[i];
      DU[stbiw__jpg_ZigZag[i]] = (int)(v < 0 ? v - 0.5f : v + 0.5f);
   }

   // Encode DC
   diff = DU[0] - DC;
   if (diff == 0) {
      stbiw__jpg_writeBits(bitBuf, bitCnt, HTDC[0]);
   } else {
      unsigned short bits[2];
      stbiw__jpg_calcBits(diff, bits);
      stbiw__jpg_writeBits(bitBuf, bitCnt, HTDC[bits[1]]);
      stbiw__jpg_writeBits(bitBuf, bitCnt, bits);
   }
   // Encode ACs
   end0pos = 63;
   for(; (end0pos>0)&&(DU[end0pos]==0); --end0pos);
   // end0pos = first element in reverse order !=0
   if(end0pos == 0) {
      stbiw__jpg_writeBits(bitBuf, bitCnt, EOB);
      return DU[0];
   }
   for(i = 1; i <= end0pos; ++i) {
      int startpos = i;
      int nrzeroes;
      unsigned short bits[2];
      for (; DU[i]==0 && i<=end0pos; ++i);
      nrzeroes = i-startpos;
      if ( nrzeroes >= 16 ) {
         int lng = nrzeroes>>4;
         int nrmarker;
         for (nrmarker=1; nrmarker <= lng; ++nrmarker)
            stbiw__jpg_writeBits(bitBuf, bitCnt, M16zeroes);
         nrzeroes &= 15;
      }
      stbiw__jpg_calcBits(DU[i], bits);
      stbiw__jpg_writeBits(bitBuf, bitCnt, HTAC[(nrzeroes<<4)+bits[1]]);
      stbiw__jpg_writeBits(bitBuf, bitCnt, bits);
   }
   if(end0pos != 63) {
      stbiw__jpg_writeBits(bitBuf, bitCnt, EOB);
   }
   return DU[0];
}

static int stbi_write_jpg_core(stbi_write_func *func, void *context, int width, int height, int comp, const void *data, int quality) {
   // Constants for encoding
   static const unsigned char std_dc_luminance_nrcodes[] = {0,0,1,5,1,1,1,1,1,1,0,0,0,0,0,0,0};
   static const unsigned char std_dc_luminance_values[] = {0,1,2,3,4,5,6,7,8,9,10,11};
   static const unsigned char std_ac_luminance_nrcodes[] = {0,0,2,1,3,3,2,4,3,5,5,4,4,0,0,1,0x7d};
   static const unsigned char std_ac_luminance_values[] = {
      0x01,0x02,0x03,0x00,0x04,0x11,0x05,0x12,0x21,0x31,0x41,0x06,0x13,0x51,0x61,0x07,0x22,0x71,0x14,0x32,0x81,0x91,0xa1,0x08,
      0x23,0x42,0xb1,0xc1,0x15,0x52,0xd1,0xf0,0x24,0x33,0x62,0x72,0x82,0x09,0x0a,0x16,0x17,0x18,0x19,0x1a,0x25,0x26,0x27,0x28,
      0x29,0x2a,0x34,0x35,0x36,0x37,0x38,0x39,0x3a,0x43,0x44,0x45,0x46,0x47,0x48,0x49,0x4a,0x53,0x54,0x55,0x56,0x57,0x58,0x59,
      0x5a,0x63,0x64,0x65,0x66,0x67,0x68,0x69,0x6a,0x73,0x74,0x75,0x76,0x77,0x78,0x79,0x7a,0x83,0x84,0x85,0x86,0x87,0x88,0x89,
      0x8a,0x92,0x93,0x94,0x95,0x96,0x97,0x98,0x99,0x9a,0xa2,0xa3,0xa4,0xa5,0xa6,0xa7,0xa8,0xa9,0xaa,0xb2,0xb3,0xb4,0xb5,0xb6,
      0xb7,0xb8,0xb9,0xba,0xc2,0xc3,0xc4,0xc5,0xc6,0xc7,0xc8,0xc9,0xca,0xd2,0xd3,0xd4,0xd5,0xd6,0xd7,0xd8,0xd9,0xda,0xe1,0xe2,
      0xe3,0xe4,0xe5,0xe6,0xe7,0xe8,0xe9,0xea,0xf1,0xf2,0xf3,0xf4,0xf5,0xf6,0xf7,0xf8,0xf9,0xfa
   };
   static const unsigned char std_dc_chrominance_nrcodes[] = {0,0,3,1,1,1,1,1,1,1,1,1,0,0,0,0,0};
   static const unsigned char std_dc_chrominance_values[] = {0,1,2,3,4,5,6,7,8,9,10,11};
   static const unsigned char std_ac_chrominance_nrcodes[] = {0,0,2,1,2,4,4,3,4,7,5,4,4,0,1,2,0x77};
   static const unsigned char std_ac_chrominance_values[] = {
      0x00,0x01,0x02,0x03,0x11,0x04,0x05,0x21,0x31,0x06,0x12,0x41,0x51,0x07,0x61,0x71,0x13,0x22,0x32,0x81,0x08,0x14,0x42,0x91,
      0xa1,0xb1,0xc1,0x09,0x23,0x33,0x52,0xf0,0x15,0x62,0x72,0xd1,0x0a,0x16,0x24,0x34,0xe1,0x25,0xf1,0x17,0x18,0x19,0x1a,0x26,
      0x27,0x28,0x29,0x2a,0x35,0x36,0x37,0x38,0x39,0x3a,0x43,0x44,0x45,0x46,0x47,0x48,0x49,0x4a,0x53,0x54,0x55,0x56,0x57,0x58,
      0x59,0x5a,0x63,0x64,0x65,0x66,0x67,0x68,0x69,0x6a,0x73,0x74,0x75,0x76,0x77,0x78,0x79,0x7a,0x82,0x83,0x84,0x85,0x86,0x87,
      0x88,0x89,0x8a,0x92,0x93,0x94,0x95,0x96,0x97,0x98,0x99,0x9a,0xa2,0xa3,0xa4,0xa5,0xa6,0xa7,0xa8,0xa9,0xaa,0xb2,0xb3,0xb4,
      0xb5,0xb6,0xb7,0xb8,0xb9,0xba,0xc2,0xc3,0xc4,0xc5,0xc6,0xc7,0xc8,0xc9,0xca,0xd2,0xd3,0xd4,0xd5,0xd6,0xd7,0xd8,0xd9,0xda,
      0xe2,0xe3,0xe4,0xe5,0xe6,0xe7,0xe8,0xe9,0xea,0xf2,0xf3,0xf4,0xf5,0xf6,0xf7,0xf8,0xf9,0xfa
   };
   // Huffman tables
   static const unsigned short YDC_HT[256][2] = { {0,2},{2,3},{3,3},{4,3},{5,3},{6,3},{14,4},{30,5},{62,6},{126,7},{254,8},{510,9}};
   static const unsigned short UVDC_HT[256][2] = { {0,2},{1,2},{2,2},{6,3},{14,4},{30,5},{62,6},{126,7},{254,8},{510,9},{1022,10},{2046,11}};
   static const unsigned short YAC_HT[256][2] = {
      {0x000A, 4},{0x0000, 2},{0x0001, 2},{0x0004, 3},{0x000B, 4},{0x001A, 5},{0x0078, 7},{0x00F8, 8},{0x03F6,10},{0xFF82,16},{0xFF83,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x000C, 4},{0x001B, 5},{0x0079, 7},{0x01F6, 9},{0x07F6,11},{0xFF84,16},{0xFF85,16},{0xFF86,16},{0xFF87,16},{0xFF88,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x001C, 5},{0x00F9, 8},{0x03F7,10},{0x0FF4,12},{0xFF89,16},{0xFF8A,16},{0xFF8B,16},{0xFF8C,16},{0xFF8D,16},{0xFF8E,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x003A, 6},{0x01F7, 9},{0x0FF5,12},{0xFF8F,16},{0xFF90,16},{0xFF91,16},{0xFF92,16},{0xFF93,16},{0xFF94,16},{0xFF95,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x003B, 6},{0x03F8,10},{0xFF96,16},{0xFF97,16},{0xFF98,16},{0xFF99,16},{0xFF9A,16},{0xFF9B,16},{0xFF9C,16},{0xFF9D,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x007A, 7},{0x07F7,11},{0xFF9E,16},{0xFF9F,16},{0xFFA0,16},{0xFFA1,16},{0xFFA2,16},{0xFFA3,16},{0xFFA4,16},{0xFFA5,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x007B, 7},{0x0FF6,12},{0xFFA6,16},{0xFFA7,16},{0xFFA8,16},{0xFFA9,16},{0xFFAA,16},{0xFFAB,16},{0xFFAC,16},{0xFFAD,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x00FA, 8},{0x0FF7,12},{0xFFAE,16},{0xFFAF,16},{0xFFB0,16},{0xFFB1,16},{0xFFB2,16},{0xFFB3,16},{0xFFB4,16},{0xFFB5,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x01F8, 9},{0x7FC0,15},{0xFFB6,16},{0xFFB7,16},{0xFFB8,16},{0xFFB9,16},{0xFFBA,16},{0xFFBB,16},{0xFFBC,16},{0xFFBD,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x01F9, 9},{0xFFBE,16},{0xFFBF,16},{0xFFC0,16},{0xFFC1,16},{0xFFC2,16},{0xFFC3,16},{0xFFC4,16},{0xFFC5,16},{0xFFC6,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x01FA, 9},{0xFFC7,16},{0xFFC8,16},{0xFFC9,16},{0xFFCA,16},{0xFFCB,16},{0xFFCC,16},{0xFFCD,16},{0xFFCE,16},{0xFFCF,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x03F9,10},{0xFFD0,16},{0xFFD1,16},{0xFFD2,16},{0xFFD3,16},{0xFFD4,16},{0xFFD5,16},{0xFFD6,16},{0xFFD7,16},{0xFFD8,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x03FA,10},{0xFFD9,16},{0xFFDA,16},{0xFFDB,16},{0xFFDC,16},{0xFFDD,16},{0xFFDE,16},{0xFFDF,16},{0xFFE0,16},{0xFFE1,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x07F8,11},{0xFFE2,16},{0xFFE3,16},{0xFFE4,16},{0xFFE5,16},{0xFFE6,16},{0xFFE7,16},{0xFFE8,16},{0xFFE9,16},{0xFFEA,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0xFFEB,16},{0xFFEC,16},{0xFFED,16},{0xFFEE,16},{0xFFEF,16},{0xFFF0,16},{0xFFF1,16},{0xFFF2,16},{0xFFF3,16},{0xFFF4,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0x07F9,11},{0xFFF5,16},{0xFFF6,16},{0xFFF7,16},{0xFFF8,16},{0xFFF9,16},{0xFFFA,16},{0xFFFB,16},{0xFFFC,16},{0xFFFD,16},{0xFFFE,16},
   };
   static const unsigned short UVAC_HT[256][2] = {
      {0x0000, 2},{0x0001, 2},{0x0004, 3},{0x000A, 4},{0x0018, 5},{0x0019, 5},{0x0038, 6},{0x0078, 7},{0x01F4, 9},{0x03F6,10},{0x0FF4,12},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x000B, 4},{0x0039, 6},{0x00F6, 8},{0x01F5, 9},{0x07F6,11},{0x0FF5,12},{0xFF88,16},{0xFF89,16},{0xFF8A,16},{0xFF8B,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x001A, 5},{0x00F7, 8},{0x03F7,10},{0x0FF6,12},{0x7FC2,15},{0xFF8C,16},{0xFF8D,16},{0xFF8E,16},{0xFF8F,16},{0xFF90,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x001B, 5},{0x00F8, 8},{0x03F8,10},{0x0FF7,12},{0xFF91,16},{0xFF92,16},{0xFF93,16},{0xFF94,16},{0xFF95,16},{0xFF96,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x003A, 6},{0x01F6, 9},{0xFF97,16},{0xFF98,16},{0xFF99,16},{0xFF9A,16},{0xFF9B,16},{0xFF9C,16},{0xFF9D,16},{0xFF9E,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x003B, 6},{0x03F9,10},{0xFF9F,16},{0xFFA0,16},{0xFFA1,16},{0xFFA2,16},{0xFFA3,16},{0xFFA4,16},{0xFFA5,16},{0xFFA6,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x0079, 7},{0x07F7,11},{0xFFA7,16},{0xFFA8,16},{0xFFA9,16},{0xFFAA,16},{0xFFAB,16},{0xFFAC,16},{0xFFAD,16},{0xFFAE,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x007A, 7},{0x07F8,11},{0xFFAF,16},{0xFFB0,16},{0xFFB1,16},{0xFFB2,16},{0xFFB3,16},{0xFFB4,16},{0xFFB5,16},{0xFFB6,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x00F9, 8},{0xFFB7,16},{0xFFB8,16},{0xFFB9,16},{0xFFBA,16},{0xFFBB,16},{0xFFBC,16},{0xFFBD,16},{0xFFBE,16},{0xFFBF,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x01F7, 9},{0xFFC0,16},{0xFFC1,16},{0xFFC2,16},{0xFFC3,16},{0xFFC4,16},{0xFFC5,16},{0xFFC6,16},{0xFFC7,16},{0xFFC8,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x01F8, 9},{0xFFC9,16},{0xFFCA,16},{0xFFCB,16},{0xFFCC,16},{0xFFCD,16},{0xFFCE,16},{0xFFCF,16},{0xFFD0,16},{0xFFD1,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x01F9, 9},{0xFFD2,16},{0xFFD3,16},{0xFFD4,16},{0xFFD5,16},{0xFFD6,16},{0xFFD7,16},{0xFFD8,16},{0xFFD9,16},{0xFFDA,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x01FA, 9},{0xFFDB,16},{0xFFDC,16},{0xFFDD,16},{0xFFDE,16},{0xFFDF,16},{0xFFE0,16},{0xFFE1,16},{0xFFE2,16},{0xFFE3,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x07F9,11},{0xFFE4,16},{0xFFE5,16},{0xFFE6,16},{0xFFE7,16},{0xFFE8,16},{0xFFE9,16},{0xFFEA,16},{0xFFEB,16},{0xFFEC,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0,0},{0x3FE0,14},{0xFFED,16},{0xFFEE,16},{0xFFEF,16},{0xFFF0,16},{0xFFF1,16},{0xFFF2,16},{0xFFF3,16},{0xFFF4,16},{0xFFF5,16},
      {0,0},{0,0},{0,0},{0,0},{0,0},{0x03FA,10},{0x7FC3,15},{0xFFF6,16},{0xFFF7,16},{0xFFF8,16},{0xFFF9,16},{0xFFFA,16},{0xFFFB,16},{0xFFFC,16},{0xFFFD,16},{0xFFFE,16},
   };

   static const int YQT[] = {16,11,10,16,24,40,51,61,12,12,14,19,26,58,60,55,14,13,16,24,40,57,69,56,14,17,22,29,51,87,80,62,18,22,37,56,68,109,103,77,24,35,55,64,81,104,113,92,49,64,78,87,103,121,120,101,72,92,95,98,112,100,103,99};
   static const int UVQT[] = {17,18,24,47,99,99,99,99,18,21,26,66,99,99,99,99,24,26,56,99,99,99,99,99,47,66,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99,99};
   static const float aasf[] = { 1.0f * 2.828427125f, 1.387039845f * 2.828427125f, 1.306562965f * 2.828427125f, 1.175875602f * 2.828427125f, 1.0f * 2.828427125f, 0.785694958f * 2.828427125f, 0.541196100f * 2.828427125f, 0.275899379f * 2.828427125f };

   int row, col, i, k, subsample;
   float fdtbl_Y[64], fdtbl_UV[64];
   unsigned char YTable[64], UVTable[64];

   if(!data || !width || !height || comp > 4 || comp < 1) {
      return 0;
   }

   quality = quality ? quality : 90;
   subsample = quality <= 90 ? 1 : 0;
   quality = quality < 1 ? 1 : quality > 100 ? 100 : quality;
   quality = quality < 50 ? 5000 / quality : 200 - quality * 2;

   for(i = 0; i < 64; ++i) {
      int uvti, yti = (YQT[i]*quality+50)/100;
      YTable[stbiw__jpg_ZigZag[i]] = (unsigned char) (yti < 1 ? 1 : yti > 255 ? 255 : yti);
      uvti = (UVQT[i]*quality+50)/100;
      UVTable[stbiw__jpg_ZigZag[i]] = (unsigned char) (uvti < 1 ? 1 : uvti > 255 ? 255 : uvti);
   }

   for(row = 0, k = 0; row < 8; ++row) {
      for(col = 0; col < 8; ++col, ++k) {
         fdtbl_Y[k]  = 1 / (YTable [stbiw__jpg_ZigZag[k]] * aasf[row] * aasf[col]);
         fdtbl_UV[k] = 1 / (UVTable[stbiw__jpg_ZigZag[k]] * aasf[row] * aasf[col]);
      }
   }

   // Write Headers
   {
      static const unsigned char head0[] = { 0xFF,0xD8,0xFF,0xE0,0,0x10,'J','F','I','F',0,1,1,0,0,1,0,1,0,0,0xFF,0xDB,0,0x84,0 };
      static const unsigned char head2[] = { 0xFF,0xDA,0,0xC,3,1,0,2,0x11,3,0x11,0,0x3F,0 };
      const unsigned char head1[] = { 0xFF,0xC0,0,0x11,8,(unsigned char)(height>>8),STBIW_UCHAR(height),(unsigned char)(width>>8),STBIW_UCHAR(width),
         3,1,(unsigned char)(subsample?0x22:0x11),0,2,0x11,1,3,0x11,1,0xFF,0xC4,0x01,0xA2,0 };
      func(context, (void*)head0, sizeof(head0));
      func(context, (void*)YTable, sizeof(YTable));
      func(context, (void*)"\1", 1);
      func(context, (void*)UVTable, sizeof(UVTable));
      func(context, (void*)head1, sizeof(head1));
      func(context, (void*)std_dc_luminance_nrcodes+1, sizeof(std_dc_luminance_nrcodes)-1);
      func(context, (void*)std_dc_luminance_values, sizeof(std_dc_luminance_values));
      func(context, (void*)"\x10", 1); // HTYACinfo
      func(context, (void*)std_ac_luminance_nrcodes+1, sizeof(std_ac_luminance_nrcodes)-1);
      func(context, (void*)std_ac_luminance_values, sizeof(std_ac_luminance_values));
      func(context, (void*)"\1", 1); // HTUDCinfo
      func(context, (void*)std_dc_chrominance_nrcodes+1, sizeof(std_dc_chrominance_nrcodes)-1);
      func(context, (void*)std_dc_chrominance_values, sizeof(std_dc_chrominance_values));
      func(context, (void*)"\x11", 1); // HTUACinfo
      func(context, (void*)std_ac_chrominance_nrcodes+1, sizeof(std_ac_chrominance_nrcodes)-1);
      func(context, (void*)std_ac_chrominance_values, sizeof(std_ac_chrominance_values));
      func(context, (void*)head2, sizeof(head2));
   }

   // Encode 8x8 macroblocks
   {
      int DCY=0, DCU=0, DCV=0;
      int bitBufLen = 0;
      unsigned char bitBuf[65536];
      unsigned char *bitBufP = bitBuf;
      const unsigned char *imageData = (const unsigned char *)data;
      int ofsG = comp > 2 ? 1 : 0, ofsB = comp > 2 ? 2 : 0;

      memset(bitBuf, 0, sizeof(bitBuf));

      for(int y = 0; y < height; y += 8) {
         for(int x = 0; x < width; x += 8) {
            float YDU[64], UDU[64], VDU[64];
            for(int row = y, pos = 0; row < y+8; ++row) {
               int clamped_row = (row < height) ? row : height - 1;
               for(int col = x; col < x+8; ++col, ++pos) {
                  int clamped_col = (col < width) ? col : width - 1;
                  int p = (clamped_row*width + clamped_col) * comp;
                  float r = imageData[p+0];
                  float g = imageData[p+ofsG];
                  float b = imageData[p+ofsB];
                  YDU[pos] = +0.29900f*r + 0.58700f*g + 0.11400f*b - 128;
                  UDU[pos] = -0.16874f*r - 0.33126f*g + 0.50000f*b;
                  VDU[pos] = +0.50000f*r - 0.41869f*g - 0.08131f*b;
               }
            }
            DCY = stbiw__jpg_processDU(&bitBufP, &bitBufLen, YDU, 8, fdtbl_Y, DCY, YDC_HT, YAC_HT);
            DCU = stbiw__jpg_processDU(&bitBufP, &bitBufLen, UDU, 8, fdtbl_UV, DCU, UVDC_HT, UVAC_HT);
            DCV = stbiw__jpg_processDU(&bitBufP, &bitBufLen, VDU, 8, fdtbl_UV, DCV, UVDC_HT, UVAC_HT);

            // Check buffer space and flush if needed
            if (bitBufP - bitBuf > 60000) {
               // Byte stuff and write
               unsigned char *src = bitBuf;
               unsigned char stuffed[65536*2];
               int stuffedLen = 0;
               while (src < bitBufP) {
                  stuffed[stuffedLen++] = *src;
                  if (*src == 0xFF) stuffed[stuffedLen++] = 0;
                  src++;
               }
               func(context, stuffed, stuffedLen);
               bitBufP = bitBuf;
               memset(bitBuf, 0, sizeof(bitBuf));
            }
         }
      }
      // Flush remaining bits
      {
         static const unsigned short fillBits[] = {0x7F, 7};
         stbiw__jpg_writeBits(&bitBufP, &bitBufLen, fillBits);
         // Byte stuff and write
         unsigned char *src = bitBuf;
         unsigned char *endP = bitBufLen > 0 ? bitBufP + 1 : bitBufP;
         unsigned char stuffed[65536*2];
         int stuffedLen = 0;
         while (src < endP) {
            stuffed[stuffedLen++] = *src;
            if (*src == 0xFF) stuffed[stuffedLen++] = 0;
            src++;
         }
         func(context, stuffed, stuffedLen);
      }
   }

   // Write EOI
   func(context, (void*)"\xFF\xD9", 2);
   return 1;
}

typedef struct {
   stbi_write_func *func;
   void *context;
   unsigned char buffer[65536];
   int buf_used;
} stbi__write_context;

static void stbi__write_flush(stbi__write_context *s) {
   if (s->buf_used) {
      s->func(s->context, s->buffer, s->buf_used);
      s->buf_used = 0;
   }
}

STBIWDEF int stbi_write_jpg_to_func(stbi_write_func *func, void *context, int x, int y, int comp, const void *data, int quality)
{
   return stbi_write_jpg_core(func, context, x, y, comp, data, quality);
}

#ifndef STBI_WRITE_NO_STDIO
#include <stdio.h>

static void stbi__stdio_write(void *context, void *data, int size) {
   fwrite(data,1,size,(FILE*) context);
}

STBIWDEF int stbi_write_jpg(char const *filename, int x, int y, int comp, const void *data, int quality) {
   FILE *f = fopen(filename, "wb");
   if (!f) return 0;
   int r = stbi_write_jpg_core(stbi__stdio_write, f, x, y, comp, data, quality);
   fclose(f);
   return r;
}
#endif // STBI_WRITE_NO_STDIO

#endif // STB_IMAGE_WRITE_IMPLEMENTATION

/*
------------------------------------------------------------------------------
This software is available under 2 licenses -- choose whichever you prefer.
------------------------------------------------------------------------------
ALTERNATIVE A - MIT License
Copyright (c) 2017 Sean Barrett
Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:
The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
------------------------------------------------------------------------------
ALTERNATIVE B - Public Domain (www.unlicense.org)
This is free and unencumbered software released into the public domain.
Anyone is free to copy, modify, publish, use, compile, sell, or distribute this
software, either in source code form or as a compiled binary, for any purpose,
commercial or non-commercial, and by any means.
In jurisdictions that recognize copyright laws, the author or authors of this
software dedicate any and all copyright interest in the software to the public
domain. We make this dedication for the benefit of the public at large and to
the detriment of our heirs and successors. We intend this dedication to be an
overt act of relinquishment in perpetuity of all present and future rights to
this software under copyright law.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
------------------------------------------------------------------------------
*/
