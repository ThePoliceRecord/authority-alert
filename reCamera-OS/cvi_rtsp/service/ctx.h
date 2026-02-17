#pragma once

#include <iostream>
#include <pthread.h>
#include <sample_comm.h>
#include <cvi_venc.h>
#include <c_apis/tdl_sdk.h>
#include <cvi_rtsp/rtsp.h>


#define MAX_PATH_LEN 256
#define topK 5
#define TDL_SUPPORTED_MODEL_FACE TDL_MODEL_SCRFD_DET_FACE
#define TDL_SUPPORTED_MODEL_PQ TDL_MODEL_CLS_ISP_SCENE

typedef TDLHandle (*AI_CreateHandle)(const int32_t);
typedef int (*AI_DestroyHandle)(void *);
typedef int (*AI_OpenModel)(void *, TDLModel, const char *, const char *);
typedef int (*AI_CloseModel)(void *, TDLModel);
typedef int (*AI_RetinaFace)(const void *, TDLModel, TDLImage, TDLFace *);
typedef TDLImage (*AI_WrapVPSSFrame)(void *, bool);
typedef int (*AI_FaceDrawRect)(TDLFace *, void *, const bool, TDLBrush);
typedef int (*AI_Image_Cls)(const TDLHandle, TDLModel, TDLImage, TDLIspMeta *, TDLClass *);
typedef int (*AI_ObjectWriteText)(char *, int, int, VIDEO_FRAME_INFO_S *, float, float, float);
typedef CVI_S32 (*AI_FreeFaceMeta)(TDLFace *face_meta);
typedef CVI_S32 (*AI_FreeClassMeta)(TDLClass *cls_meta);
typedef CVI_S32 (*AI_FreeVPSSFrame)(TDLImage image_handle);


struct SERVICE_CTX_ENTITY {
    pthread_t worker;
    pthread_t teaisppq_worker;
    bool running;
    pthread_mutex_t mutex;

    CVI_RTSP_SESSION *rtspSession;
    uint32_t rtsp_bitrate;
    char rtspURL[32];

    bool bVpssBinding;
    bool bVencBindVpss;
    bool bEnable3DNR;
    bool bEnableLSC;
    bool enableIspInfoOsd;

    int src_width;
    int src_height;
    int dst_width;
    int dst_height;

    uint32_t buf_blk_cnt;

    uint64_t vpssPrePTS;
    uint64_t vencPrePTS;

    COMPRESS_MODE_E compress_mode;
    VI_CHN ViChn;
    VI_PIPE ViPipe;
    VPSS_CHN VpssChn;
    VPSS_GRP VpssGrp;
    VENC_CHN VencChn;
    chnInputCfg venc_cfg;
    char venc_json[MAX_PATH_LEN];
    void *ctx;
    bool enableRetinaFace;
    bool enableTeaisppq;
    bool enableTEAISPBnr;
    char teaisp_model_list[MAX_PATH_LEN];
    uint8_t tpu_device_id;
    TDLHandle ai_handle;
    AI_CreateHandle ai_create_handle;
    AI_DestroyHandle ai_destroy_handle;
    AI_OpenModel ai_open_model;
    AI_CloseModel ai_close_model;
    AI_RetinaFace ai_retinaface;
    AI_WrapVPSSFrame ai_wrap_vpss_frame;
    AI_FaceDrawRect ai_face_draw_rect;
    AI_ObjectWriteText ai_write_text;
    TDLHandle teaisppq_handle;
    AI_Image_Cls ai_img_cls;
    AI_FreeClassMeta ai_free_class_meta;
	AI_FreeVPSSFrame ai_free_vpss_frame;
    AI_FreeFaceMeta ai_free_face_meta;
};

#define SERVICE_CTX_ENTITY_MAX_NUM 2

struct SERVICE_SBM {
	uint8_t enable;
	uint16_t bufLine;
	uint8_t bufSize;
};

struct REPLAY_PARAM {
	uint8_t pixelFormat;
	uint16_t width;
	uint16_t height;
	bool timingEnable;
	uint32_t frameRate;
	uint8_t WDRMode;
	uint8_t bayerFormat;
	COMPRESS_MODE_E compressMode;
};

struct SERVICE_CTX {
    SERVICE_CTX_ENTITY entity[SERVICE_CTX_ENTITY_MAX_NUM];
    CVI_RTSP_CTX *rtspCtx;
    int rtspPort;
    uint32_t rtsp_max_buf_size;
    SAMPLE_VI_CONFIG_S stViConfig;
    SAMPLE_INI_CFG_S stIniCfg;
    uint8_t rtsp_num;
    uint8_t dev_num;
    VI_VPSS_MODE_E vi_vpss_mode;
    uint32_t buf1_blk_cnt;
    uint8_t max_use_tpu_num;
    char model_path[MAX_PATH_LEN];
    char teaisppq_model_path[MAX_PATH_LEN];
    void *ai_dl;
    SERVICE_SBM sbm;
    int tdl_ref_count;
    /*isp*/
    bool enable_set_sensor_config;
    char sensor_config_path[MAX_PATH_LEN];
    uint8_t isp_debug_lvl;
    bool enable_set_pq_bin;
    char sdr_pq_bin_path[MAX_PATH_LEN];
    char wdr_pq_bin_path[MAX_PATH_LEN];
    bool replayMode;
    REPLAY_PARAM replayParam;
    // teaisppq
    pthread_mutex_t teaisppq_mutex;
    pthread_cond_t teaisppq_cond;
    int teaisppq_turn_pipe;
    unsigned int sensor_number;
    bool teaisppq_on_status_ls[SERVICE_CTX_ENTITY_MAX_NUM];
};
