/*
 * The wlroots 0.18 package exposes wlr_output_power_management_v1.h but does
 * not install this generated protocol header. Only the mode enum is part of
 * the public wlroots structure used by HatWM; protocol object definitions are
 * owned by libwlroots.
 */
#ifndef WLR_OUTPUT_POWER_MANAGEMENT_UNSTABLE_V1_PROTOCOL_H
#define WLR_OUTPUT_POWER_MANAGEMENT_UNSTABLE_V1_PROTOCOL_H

enum zwlr_output_power_v1_mode {
	ZWLR_OUTPUT_POWER_V1_MODE_OFF = 0,
	ZWLR_OUTPUT_POWER_V1_MODE_ON = 1,
};

#endif
