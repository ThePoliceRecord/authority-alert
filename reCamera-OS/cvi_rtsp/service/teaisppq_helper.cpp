#include "ctx.h"
#include "vpss_helper.h"
#include <dlfcn.h>
#include <sys/stat.h>
#include <sys/prctl.h>
#include <unistd.h>
#include "cvi_isp.h"
#include <string>
#include <vector>
#include <iostream>
#include <fstream>
#include <sstream>
#include <string.h>
#include "cvi_isp.h"
#include "cvi_comm_isp.h"
#include "utils.h"
#include <chrono>
#include <math.h>
#include "cvi_awb.h"

#define TEAISPPQ_LIB "libcvi_tdl.so"
#define USE_VI_RAW
#define TEAISP_PQ_SLEEP_TIME 200000

#define LOAD_SYMBOL(dl, sym, type, fn) \
do { \
	if (NULL == fn && NULL == (fn = (type)dlsym(dl, sym))) { \
		printf("load symbol %s fail, %s\n", sym, dlerror()); \
		return -1; \
	} \
} while(0);

static const char *teaisppq_scene_str[5] = {
	"SNOW",
	"FOG",
	"BACKLIGHT",
	"GRASS",
	"COMMON"
};

static CVI_U64 inference_count;

static int load_teaisppq_symbol(SERVICE_CTX *ctx)
{
	if (!ctx->ai_dl) {
		std::cout << "Loading " TEAISPPQ_LIB " ..." << std::endl;
		ctx->ai_dl = dlopen(TEAISPPQ_LIB, RTLD_LAZY);

		if (!ctx->ai_dl) {
			std::cout << "dlopen " TEAISPPQ_LIB " fail: " << std::endl;
			std::cout << dlerror() << std::endl;
			return -1;
		}
	}

	for (int idx=0; idx<ctx->dev_num; idx++) {
		SERVICE_CTX_ENTITY *ent = &ctx->entity[idx];
		// common
		LOAD_SYMBOL(ctx->ai_dl, "TDL_CreateHandle", AI_CreateHandle, ent->ai_create_handle);
		LOAD_SYMBOL(ctx->ai_dl, "TDL_DestroyHandle", AI_DestroyHandle, ent->ai_destroy_handle);
		LOAD_SYMBOL(ctx->ai_dl, "TDL_OpenModel", AI_OpenModel, ent->ai_open_model);
		LOAD_SYMBOL(ctx->ai_dl, "TDL_CloseModel", AI_CloseModel, ent->ai_close_model);
		LOAD_SYMBOL(ctx->ai_dl, "TDL_WrapVPSSFrame", AI_WrapVPSSFrame, ent->ai_wrap_vpss_frame);
		LOAD_SYMBOL(ctx->ai_dl, "TDL_DestroyImage", AI_FreeVPSSFrame, ent->ai_free_vpss_frame);
		// scene classification
		LOAD_SYMBOL(ctx->ai_dl, "TDL_IspClassification", AI_Image_Cls, ent->ai_img_cls);
		LOAD_SYMBOL(ctx->ai_dl, "TDL_ObjectWriteText", AI_ObjectWriteText, ent->ai_write_text);
		LOAD_SYMBOL(ctx->ai_dl, "TDL_ReleaseClassMeta", AI_FreeClassMeta, ent->ai_free_class_meta);

	}

	return 0;
}

static void set_teaisppq_scene(SERVICE_CTX_ENTITY *ent, TDLClass &cls_meta)
{
	TEAISP_PQ_SCENE_INFO scene_info;

	switch (cls_meta.info[0].class_id) {
	case 0:
		scene_info.scene = SCENE_SNOW;
		break;
	case 1:
		scene_info.scene = SCENE_FOG;
		break;
	case 2:
		scene_info.scene = SCENE_BACKLIGHT;
		break;
	case 3:
		scene_info.scene = SCENE_GRASS;
		break;
	default:
		scene_info.scene = SCENE_COMMON;
		break;
	}

	float score = cls_meta.info[0].score;
	scene_info.scene_score = score * 100;

	CVI_TEAISP_PQ_SetSceneInfo(ent->ViPipe, &scene_info);
}

int run_teaisppq_vi(SERVICE_CTX_ENTITY *ent)
{
	int ret = 0;
	int frmNum = 1;
	std::string ret_name;
	TDLClass cls_meta = {0};
	VI_PIPE pipe = ent->ViPipe;
	VIDEO_FRAME_INFO_S stVideoFrame[2] = {};
	PQ_PARAMETER_S pq_param;
	TDLIspMeta isp_pq;

	if (0x00 != get_vi_raw(pipe, stVideoFrame, &frmNum)) {
		printf("[TEAISP.pq] get vi raw failed...\n");
		return -1;
	}

	get_pq_parameter(ent->ViPipe, &pq_param);

	isp_pq.awb[0] = pq_param.awb_rgain;
	isp_pq.awb[1] = pq_param.awb_ggain;
	isp_pq.awb[2] = pq_param.awb_bgain;

	for (int i = 0; i < 9; i++) {
		isp_pq.ccm[i] = pq_param.stInnerStateInfo.ccm[i];
	}

	isp_pq.blc = pq_param.stInnerStateInfo.blcOffsetR;
	isp_pq.blc += pq_param.stInnerStateInfo.blcOffsetGr;
	isp_pq.blc += pq_param.stInnerStateInfo.blcOffsetGb;
	isp_pq.blc += pq_param.stInnerStateInfo.blcOffsetB;
	isp_pq.blc /= 4;

	cls_meta.size = topK;
	cls_meta.info = (TDLClassInfo *) malloc(topK * sizeof(TDLClassInfo));

	// inference
	// TODO: force set the 1, ignore the se frame
	frmNum = 1;
	for (int i = 0; i < frmNum; i++) {
		++inference_count;
		//std::cout << "pipe: " << pipe << ", ---------- do the inference cnt: " << ++inference_count << " ----------" << std::endl;

		auto start_time = std::chrono::high_resolution_clock::now();
		//stVideoFrame[i].stVFrame.u32Width *= 1.5;
		TDLImage vpss_image = ent->ai_wrap_vpss_frame(stVideoFrame + i, false);
		ret = ent->ai_img_cls(ent->teaisppq_handle, TDL_SUPPORTED_MODEL_PQ, vpss_image, &isp_pq, &cls_meta);
		//stVideoFrame[i].stVFrame.u32Width /= 1.5;
		auto end_time = std::chrono::high_resolution_clock::now();
		auto duration_ms = std::chrono::duration_cast<std::chrono::milliseconds>(end_time - start_time);
		//std::cout << "inference time: " << duration_ms.count() << " ms" << std::endl;

		if (ret != CVI_SUCCESS) {
			std::cout << "Error: inference return type: " << ret << std::endl;
			ent->ai_free_class_meta(&cls_meta);
			goto INFERENCE_FAIL;
		}
		ent->ai_free_vpss_frame(vpss_image);
	}

	set_teaisppq_scene(ent, cls_meta);

	// dump raw, keep here
	if (access("dump_raw_12_bits", F_OK) == 0 || access("dump_raw_once", F_OK) == 0) {
		// string format: scene-0_20-1_20-2_20-3_20-4_20-rg_1024-gg_1024-bg_1024-cnt_1.raw
		int int_cls_score[5] = {0};
		std::string ret_name = teaisppq_scene_str[cls_meta.info[0].class_id] + std::string("-");

		for (int i = 0; i < 5; ++i) {
			int_cls_score[cls_meta.info[i].class_id] = cls_meta.info[i].score * 100;
		}

		for (int i = 0; i < 5; ++i) {
			ret_name += std::to_string(i) + "_" + std::to_string(int_cls_score[i]);
			if (i != 4) {
				ret_name += "-";
			}
		}
		ret_name += "-rg" + std::to_string(isp_pq.awb[0]);
		ret_name += "-gg" + std::to_string(isp_pq.awb[1]);
		ret_name += "-bg" + std::to_string(isp_pq.awb[2]);
		ret_name +=  "-cnt" + std::to_string(inference_count) + ".raw";
		dump_raw_with_ret(stVideoFrame, frmNum, ret_name.c_str(), RAW_12_BIT);
		std::cout << "dump raw: " << ret_name << std::endl;

		if (access("dump_raw_once", F_OK) == 0) {
			system("rm dump_raw_once");
		}

	} else if (access("dump_raw_16_bits", F_OK) == 0) {
		dump_raw(stVideoFrame, frmNum, RAW_16_BIT);
	}

	ent->ai_free_class_meta(&cls_meta);

INFERENCE_FAIL:
	// release raw
	CVI_VI_ReleasePipeFrame(pipe, stVideoFrame);

	return ret;
}

int init_teaisppq(SERVICE_CTX *ctx)
{
	int ret = 0;

	// init the sensor num after vi init
	CVI_VI_GetDevNum(&ctx->sensor_number);

	for (int idx = 0; idx < ctx->dev_num; idx++) {
		SERVICE_CTX_ENTITY *ent = &ctx->entity[idx];

		ent->ViPipe = idx;

		if (!ent->enableTeaisppq) {
			continue;
		}

		ctx->teaisppq_on_status_ls[idx] = 1;
		ctx->teaisppq_turn_pipe = idx;

		if (0 > load_teaisppq_symbol(ctx)) {
			printf("load_teaisppq_symbol fail\n");
		}

		ent->teaisppq_handle = ent->ai_create_handle(ent->tpu_device_id);
		if (ent->teaisppq_handle == NULL) {
			printf("Create teaisppq handle failed!\n");
			return -1;
		}

		// open model
		struct stat st = {0};

		if (0 == strlen(ctx->teaisppq_model_path) || 0 != stat(ctx->teaisppq_model_path, &st)) {
			printf("scene classification model %s not found\n", ctx->teaisppq_model_path);
			return -1;
		}

		if (ent->ai_open_model(ent->teaisppq_handle, TDL_SUPPORTED_MODEL_PQ,
			ctx->teaisppq_model_path, NULL) != CVI_SUCCESS) {
			printf("CVI_AI_SetModelPath: %s failed!\n", ctx->teaisppq_model_path);
			return -1;
		}
	}

	ctx->tdl_ref_count++;

	return ret;
}

void deinit_teaisppq(SERVICE_CTX *ctx)
{
	for (int idx = 0; idx < ctx->dev_num; idx++) {
		SERVICE_CTX_ENTITY *ent = &ctx->entity[idx];

		if (!ent->enableTeaisppq) {
			continue;
		}

		if (ent->teaisppq_handle != NULL) {
			ent->ai_close_model(ent->teaisppq_handle, TDL_SUPPORTED_MODEL_PQ);
			ent->ai_destroy_handle(ent->teaisppq_handle);
		}
	}

	if (ctx->ai_dl && --ctx->tdl_ref_count <= 0) {
		dlclose(ctx->ai_dl);
		ctx->ai_dl = NULL;
	}
}

void *teaisppqTask(void *arg)
{
	int rc = 0;
	SERVICE_CTX_ENTITY *ent = (SERVICE_CTX_ENTITY *)arg;

	prctl(PR_SET_NAME, "AIPQ", 0, 0, 0);

	while (ent->running && ent->enableTeaisppq) {
#ifdef USE_VI_RAW
		if (access("STOP_AI", F_OK) == 0) {
			if (access("START_AI", F_OK) == 0) {
				system("rm -f STOP_AI");
			} else {
				printf("touch START_AI to run AIPQ...\n");
			}
			usleep(1 * 1000000);
		} else {
			SERVICE_CTX *ctx = (SERVICE_CTX *)ent->ctx;
			pthread_mutex_lock(&ctx->teaisppq_mutex);

			while (ctx->teaisppq_turn_pipe != ent->ViPipe) {
				pthread_cond_wait(&ctx->teaisppq_cond, &ctx->teaisppq_mutex);
			}
			rc = run_teaisppq_vi(ent);
			// update teaisppq_turn_pipe
			for (int i = 1; i <= VI_MAX_PIPE_NUM; i++) {
				int next_pipe = (ent->ViPipe + i) % VI_MAX_PIPE_NUM;
				if (ctx->teaisppq_on_status_ls[next_pipe] == 1) {
					ctx->teaisppq_turn_pipe = next_pipe;
					break;
				}
			}

			pthread_cond_broadcast(&ctx->teaisppq_cond);
			pthread_mutex_unlock(&ctx->teaisppq_mutex);
		}
		usleep(TEAISP_PQ_SLEEP_TIME);
#else
		//std::cout << "not run teaisppq ..." << std::endl;
#endif
		if (0 != rc) {
			std::cout << "fail to run the teaisppq" << std::endl;
			if (access("STOP_AI", F_OK) == 0)
				break;
		}
	}

	return NULL;
}

