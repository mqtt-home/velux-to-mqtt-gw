package protocol

import "strconv"

// Command codes (GW_*). Ported byte-exact from const.Command.
const (
	GW_ERROR_NTF  Command = 0x0000
	GW_REBOOT_REQ Command = 0x0001
	GW_REBOOT_CFM Command = 0x0002

	GW_SET_FACTORY_DEFAULT_REQ Command = 0x0003
	GW_SET_FACTORY_DEFAULT_CFM Command = 0x0004

	GW_GET_VERSION_REQ Command = 0x0008
	GW_GET_VERSION_CFM Command = 0x0009

	GW_GET_PROTOCOL_VERSION_REQ Command = 0x000A
	GW_GET_PROTOCOL_VERSION_CFM Command = 0x000B

	GW_GET_STATE_REQ Command = 0x000C
	GW_GET_STATE_CFM Command = 0x000D

	GW_LEAVE_LEARN_STATE_REQ Command = 0x000E
	GW_LEAVE_LEARN_STATE_CFM Command = 0x000F

	GW_GET_NETWORK_SETUP_REQ Command = 0x00E0
	GW_GET_NETWORK_SETUP_CFM Command = 0x00E1
	GW_SET_NETWORK_SETUP_REQ Command = 0x00E2
	GW_SET_NETWORK_SETUP_CFM Command = 0x00E3

	GW_CS_GET_SYSTEMTABLE_DATQ_REQ Command = 0x0100
	GW_CS_GET_SYSTEMTABLE_DATA_CFM Command = 0x0101
	GW_CS_GET_SYSTEMTABLE_DATA_NTF Command = 0x0102

	GW_CS_DISCOVER_NODES_REQ Command = 0x0103
	GW_CS_DISCOVER_NODES_CFM Command = 0x0104
	GW_CS_DISCOVER_NODES_NTF Command = 0x0105

	GW_CS_REMOVE_NODES_REQ Command = 0x0106
	GW_CS_REMOVE_NODES_CFM Command = 0x0107

	GW_CS_VIRGIN_STATE_REQ Command = 0x0108
	GW_CS_VIRGIN_STATE_CFM Command = 0x0109

	GW_CS_CONTROLLER_COPY_REQ        Command = 0x010A
	GW_CS_CONTROLLER_COPY_CFM        Command = 0x010B
	GW_CS_CONTROLLER_COPY_NTF        Command = 0x010C
	GW_CS_CONTROLLER_COPY_CANCEL_NTF Command = 0x010D

	GW_CS_RECEIVE_KEY_REQ Command = 0x010E
	GW_CS_RECEIVE_KEY_CFM Command = 0x010F
	GW_CS_RECEIVE_KEY_NTF Command = 0x0110

	GW_CS_PGC_JOB_NTF             Command = 0x0111
	GW_CS_SYSTEM_TABLE_UPDATE_NTF Command = 0x0112
	GW_CS_GENERATE_NEW_KEY_REQ    Command = 0x0113
	GW_CS_GENERATE_NEW_KEY_CFM    Command = 0x0114
	GW_CS_GENERATE_NEW_KEY_NTF    Command = 0x0115

	GW_CS_REPAIR_KEY_REQ Command = 0x0116
	GW_CS_REPAIR_KEY_CFM Command = 0x0117
	GW_CS_REPAID_KEY_NTF Command = 0x0118

	GW_CS_ACTIVATE_CONFIGURATION_MODE_REQ Command = 0x0119
	GW_CS_ACTIVATE_CONFIGURATION_MODE_CFM Command = 0x011A

	GW_GET_NODE_INFORMATION_REQ Command = 0x0200
	GW_GET_NODE_INFORMATION_CFM Command = 0x0201
	GW_GET_NODE_INFORMATION_NTF Command = 0x0210

	GW_GET_ALL_NODES_INFORMATION_REQ          Command = 0x0202
	GW_GET_ALL_NODES_INFORMATION_CFM          Command = 0x0203
	GW_GET_ALL_NODES_INFORMATION_NTF          Command = 0x0204
	GW_GET_ALL_NODES_INFORMATION_FINISHED_NTF Command = 0x0205

	GW_SET_NODE_VARIATION_REQ Command = 0x0206
	GW_SET_NODE_VARIATION_CFM Command = 0x0207

	GW_SET_NODE_NAME_REQ Command = 0x0208
	GW_SET_NODE_NAME_CFM Command = 0x0209

	GW_SET_NODE_VELOCITY_REQ Command = 0x020A
	GW_SET_NODE_VELOCITY_CFM Command = 0x020B

	GW_NODE_INFORMATION_CHANGED_NTF Command = 0x020C

	GW_NODE_STATE_POSITION_CHANGED_NTF Command = 0x0211

	GW_SET_NODE_ORDER_AND_PLACEMENT_REQ Command = 0x020D
	GW_SET_NODE_ORDER_AND_PLACEMENT_CFM Command = 0x020E

	GW_GET_GROUP_INFORMATION_REQ Command = 0x0220
	GW_GET_GROUP_INFORMATION_CFM Command = 0x0221
	GW_GET_GROUP_INFORMATION_NTF Command = 0x0230

	GW_SET_GROUP_INFORMATION_REQ Command = 0x0222
	GW_SET_GROUP_INFORMATION_CFM Command = 0x0223

	GW_GROUP_INFORMATION_CHANGED_NTF Command = 0x0224

	GW_DELETE_GROUP_REQ Command = 0x0225
	GW_DELETE_GROUP_CFM Command = 0x0226

	GW_NEW_GROUP_REQ Command = 0x0227
	GW_NEW_GROUP_CFM Command = 0x0228

	GW_GET_ALL_GROUPS_INFORMATION_REQ Command = 0x0229
	GW_GET_ALL_GROUPS_INFORMATION_CFM Command = 0x022A
	GW_GET_ALL_GROUPS_INFORMATION_NTF Command = 0x022B

	GW_GET_ALL_GROUPS_INFORMATION_FINISHED_NTF Command = 0x022C

	GW_GROUP_DELETED_NTF Command = 0x022D

	GW_HOUSE_STATUS_MONITOR_ENABLE_REQ Command = 0x0240
	GW_HOUSE_STATUS_MONITOR_ENABLE_CFM Command = 0x0241

	GW_HOUSE_STATUS_MONITOR_DISABLE_REQ Command = 0x0242
	GW_HOUSE_STATUS_MONITOR_DISABLE_CFM Command = 0x0243

	GW_COMMAND_SEND_REQ           Command = 0x0300
	GW_COMMAND_SEND_CFM           Command = 0x0301
	GW_COMMAND_RUN_STATUS_NTF     Command = 0x0302
	GW_COMMAND_REMAINING_TIME_NTF Command = 0x0303
	GW_SESSION_FINISHED_NTF       Command = 0x0304

	GW_STATUS_REQUEST_REQ Command = 0x0305
	GW_STATUS_REQUEST_CFM Command = 0x0306
	GW_STATUS_REQUEST_NTF Command = 0x0307

	GW_WINK_SEND_REQ Command = 0x0308
	GW_WINK_SEND_CFM Command = 0x0309
	GW_WINK_SEND_NTF Command = 0x030A

	GW_SET_LIMITATION_REQ        Command = 0x0310
	GW_SET_LIMITATION_CFM        Command = 0x0311
	GW_GET_LIMITATION_STATUS_REQ Command = 0x0312
	GW_GET_LIMITATION_STATUS_CFM Command = 0x0313
	GW_LIMITATION_STATUS_NTF     Command = 0x0314

	GW_MODE_SEND_REQ Command = 0x0320
	GW_MODE_SEND_CFM Command = 0x0321
	GW_MODE_SEND_NTF Command = 0x0322

	GW_INITIALIZE_SCENE_REQ        Command = 0x0400
	GW_INITIALIZE_SCENE_CFM        Command = 0x0401
	GW_INITIALIZE_SCENE_NTF        Command = 0x0402
	GW_INITIALIZE_SCENE_CANCEL_REQ Command = 0x0403
	GW_INITIALIZE_SCENE_CANCEL_CFM Command = 0x0404
	GW_RECORD_SCENE_REQ            Command = 0x0405
	GW_RECORD_SCENE_CFM            Command = 0x0406
	GW_RECORD_SCENE_NTF            Command = 0x0407

	GW_DELETE_SCENE_REQ Command = 0x0408
	GW_DELETE_SCENE_CFM Command = 0x0409

	GW_RENAME_SCENE_REQ Command = 0x040A
	GW_RENAME_SCENE_CFM Command = 0x040B

	GW_GET_SCENE_LIST_REQ Command = 0x040C
	GW_GET_SCENE_LIST_CFM Command = 0x040D
	GW_GET_SCENE_LIST_NTF Command = 0x040E

	GW_GET_SCENE_INFORMATION_REQ Command = 0x040F
	GW_GET_SCENE_INFORMATION_CFM Command = 0x0410
	GW_GET_SCENE_INFORMATION_NTF Command = 0x0411

	GW_ACTIVATE_SCENE_REQ Command = 0x0412
	GW_ACTIVATE_SCENE_CFM Command = 0x0413

	GW_STOP_SCENE_REQ Command = 0x0415
	GW_STOP_SCENE_CFM Command = 0x0416

	GW_SCENE_INFORMATION_CHANGED_NTF Command = 0x0419

	GW_ACTIVATE_PRODUCTGROUP_REQ Command = 0x0447
	GW_ACTIVATE_PRODUCTGROUP_CFM Command = 0x0448
	GW_ACTIVATE_PRODUCTGROUP_NTF Command = 0x0449

	GW_GET_CONTACT_INPUT_LINK_LIST_REQ Command = 0x0460
	GW_GET_CONTACT_INPUT_LINK_LIST_CFM Command = 0x0461

	GW_SET_CONTACT_INPUT_LINK_REQ Command = 0x0462
	GW_SET_CONTACT_INPUT_LINK_CFM Command = 0x0463

	GW_REMOVE_CONTACT_INPUT_LINK_REQ Command = 0x0464
	GW_REMOVE_CONTACT_INPUT_LINK_CFM Command = 0x0465

	GW_GET_ACTIVATION_LOG_HEADER_REQ Command = 0x0500
	GW_GET_ACTIVATION_LOG_HEADER_CFM Command = 0x0501

	GW_CLEAR_ACTIVATION_LOG_REQ Command = 0x0502
	GW_CLEAR_ACTIVATION_LOG_CFM Command = 0x0503

	GW_GET_ACTIVATION_LOG_LINE_REQ Command = 0x0504
	GW_GET_ACTIVATION_LOG_LINE_CFM Command = 0x0505

	GW_ACTIVATION_LOG_UPDATED_NTF Command = 0x0506

	GW_GET_MULTIPLE_ACTIVATION_LOG_LINES_REQ Command = 0x0507
	GW_GET_MULTIPLE_ACTIVATION_LOG_LINES_NTF Command = 0x0508
	GW_GET_MULTIPLE_ACTIVATION_LOG_LINES_CFN Command = 0x0509

	GW_SET_UTC_REQ Command = 0x2000
	GW_SET_UTC_CFM Command = 0x2001

	GW_RTC_SET_TIME_ZONE_REQ Command = 0x2002
	GW_RTC_SET_TIME_ZONE_CFM Command = 0x2003

	GW_GET_LOCAL_TIME_REQ Command = 0x2004
	GW_GET_LOCAL_TIME_CFM Command = 0x2005

	GW_PASSWORD_ENTER_REQ Command = 0x3000
	GW_PASSWORD_ENTER_CFM Command = 0x3001

	GW_PASSWORD_CHANGE_REQ Command = 0x3002
	GW_PASSWORD_CHANGE_CFM Command = 0x3003
	GW_PASSWORD_CHANGE_NTF Command = 0x3004
)

// commandNames maps command codes to their symbolic names, for String().
var commandNames = map[Command]string{
	GW_ERROR_NTF:                               "GW_ERROR_NTF",
	GW_REBOOT_REQ:                              "GW_REBOOT_REQ",
	GW_REBOOT_CFM:                              "GW_REBOOT_CFM",
	GW_SET_FACTORY_DEFAULT_REQ:                 "GW_SET_FACTORY_DEFAULT_REQ",
	GW_SET_FACTORY_DEFAULT_CFM:                 "GW_SET_FACTORY_DEFAULT_CFM",
	GW_GET_VERSION_REQ:                         "GW_GET_VERSION_REQ",
	GW_GET_VERSION_CFM:                         "GW_GET_VERSION_CFM",
	GW_GET_PROTOCOL_VERSION_REQ:                "GW_GET_PROTOCOL_VERSION_REQ",
	GW_GET_PROTOCOL_VERSION_CFM:                "GW_GET_PROTOCOL_VERSION_CFM",
	GW_GET_STATE_REQ:                           "GW_GET_STATE_REQ",
	GW_GET_STATE_CFM:                           "GW_GET_STATE_CFM",
	GW_LEAVE_LEARN_STATE_REQ:                   "GW_LEAVE_LEARN_STATE_REQ",
	GW_LEAVE_LEARN_STATE_CFM:                   "GW_LEAVE_LEARN_STATE_CFM",
	GW_GET_NETWORK_SETUP_REQ:                   "GW_GET_NETWORK_SETUP_REQ",
	GW_GET_NETWORK_SETUP_CFM:                   "GW_GET_NETWORK_SETUP_CFM",
	GW_SET_NETWORK_SETUP_REQ:                   "GW_SET_NETWORK_SETUP_REQ",
	GW_SET_NETWORK_SETUP_CFM:                   "GW_SET_NETWORK_SETUP_CFM",
	GW_CS_GET_SYSTEMTABLE_DATQ_REQ:             "GW_CS_GET_SYSTEMTABLE_DATQ_REQ",
	GW_CS_GET_SYSTEMTABLE_DATA_CFM:             "GW_CS_GET_SYSTEMTABLE_DATA_CFM",
	GW_CS_GET_SYSTEMTABLE_DATA_NTF:             "GW_CS_GET_SYSTEMTABLE_DATA_NTF",
	GW_CS_DISCOVER_NODES_REQ:                   "GW_CS_DISCOVER_NODES_REQ",
	GW_CS_DISCOVER_NODES_CFM:                   "GW_CS_DISCOVER_NODES_CFM",
	GW_CS_DISCOVER_NODES_NTF:                   "GW_CS_DISCOVER_NODES_NTF",
	GW_CS_REMOVE_NODES_REQ:                     "GW_CS_REMOVE_NODES_REQ",
	GW_CS_REMOVE_NODES_CFM:                     "GW_CS_REMOVE_NODES_CFM",
	GW_CS_VIRGIN_STATE_REQ:                     "GW_CS_VIRGIN_STATE_REQ",
	GW_CS_VIRGIN_STATE_CFM:                     "GW_CS_VIRGIN_STATE_CFM",
	GW_CS_CONTROLLER_COPY_REQ:                  "GW_CS_CONTROLLER_COPY_REQ",
	GW_CS_CONTROLLER_COPY_CFM:                  "GW_CS_CONTROLLER_COPY_CFM",
	GW_CS_CONTROLLER_COPY_NTF:                  "GW_CS_CONTROLLER_COPY_NTF",
	GW_CS_CONTROLLER_COPY_CANCEL_NTF:           "GW_CS_CONTROLLER_COPY_CANCEL_NTF",
	GW_CS_RECEIVE_KEY_REQ:                      "GW_CS_RECEIVE_KEY_REQ",
	GW_CS_RECEIVE_KEY_CFM:                      "GW_CS_RECEIVE_KEY_CFM",
	GW_CS_RECEIVE_KEY_NTF:                      "GW_CS_RECEIVE_KEY_NTF",
	GW_CS_PGC_JOB_NTF:                          "GW_CS_PGC_JOB_NTF",
	GW_CS_SYSTEM_TABLE_UPDATE_NTF:              "GW_CS_SYSTEM_TABLE_UPDATE_NTF",
	GW_CS_GENERATE_NEW_KEY_REQ:                 "GW_CS_GENERATE_NEW_KEY_REQ",
	GW_CS_GENERATE_NEW_KEY_CFM:                 "GW_CS_GENERATE_NEW_KEY_CFM",
	GW_CS_GENERATE_NEW_KEY_NTF:                 "GW_CS_GENERATE_NEW_KEY_NTF",
	GW_CS_REPAIR_KEY_REQ:                       "GW_CS_REPAIR_KEY_REQ",
	GW_CS_REPAIR_KEY_CFM:                       "GW_CS_REPAIR_KEY_CFM",
	GW_CS_REPAID_KEY_NTF:                       "GW_CS_REPAID_KEY_NTF",
	GW_CS_ACTIVATE_CONFIGURATION_MODE_REQ:      "GW_CS_ACTIVATE_CONFIGURATION_MODE_REQ",
	GW_CS_ACTIVATE_CONFIGURATION_MODE_CFM:      "GW_CS_ACTIVATE_CONFIGURATION_MODE_CFM",
	GW_GET_NODE_INFORMATION_REQ:                "GW_GET_NODE_INFORMATION_REQ",
	GW_GET_NODE_INFORMATION_CFM:                "GW_GET_NODE_INFORMATION_CFM",
	GW_GET_NODE_INFORMATION_NTF:                "GW_GET_NODE_INFORMATION_NTF",
	GW_GET_ALL_NODES_INFORMATION_REQ:           "GW_GET_ALL_NODES_INFORMATION_REQ",
	GW_GET_ALL_NODES_INFORMATION_CFM:           "GW_GET_ALL_NODES_INFORMATION_CFM",
	GW_GET_ALL_NODES_INFORMATION_NTF:           "GW_GET_ALL_NODES_INFORMATION_NTF",
	GW_GET_ALL_NODES_INFORMATION_FINISHED_NTF:  "GW_GET_ALL_NODES_INFORMATION_FINISHED_NTF",
	GW_SET_NODE_VARIATION_REQ:                  "GW_SET_NODE_VARIATION_REQ",
	GW_SET_NODE_VARIATION_CFM:                  "GW_SET_NODE_VARIATION_CFM",
	GW_SET_NODE_NAME_REQ:                       "GW_SET_NODE_NAME_REQ",
	GW_SET_NODE_NAME_CFM:                       "GW_SET_NODE_NAME_CFM",
	GW_SET_NODE_VELOCITY_REQ:                   "GW_SET_NODE_VELOCITY_REQ",
	GW_SET_NODE_VELOCITY_CFM:                   "GW_SET_NODE_VELOCITY_CFM",
	GW_NODE_INFORMATION_CHANGED_NTF:            "GW_NODE_INFORMATION_CHANGED_NTF",
	GW_NODE_STATE_POSITION_CHANGED_NTF:         "GW_NODE_STATE_POSITION_CHANGED_NTF",
	GW_SET_NODE_ORDER_AND_PLACEMENT_REQ:        "GW_SET_NODE_ORDER_AND_PLACEMENT_REQ",
	GW_SET_NODE_ORDER_AND_PLACEMENT_CFM:        "GW_SET_NODE_ORDER_AND_PLACEMENT_CFM",
	GW_GET_GROUP_INFORMATION_REQ:               "GW_GET_GROUP_INFORMATION_REQ",
	GW_GET_GROUP_INFORMATION_CFM:               "GW_GET_GROUP_INFORMATION_CFM",
	GW_GET_GROUP_INFORMATION_NTF:               "GW_GET_GROUP_INFORMATION_NTF",
	GW_SET_GROUP_INFORMATION_REQ:               "GW_SET_GROUP_INFORMATION_REQ",
	GW_SET_GROUP_INFORMATION_CFM:               "GW_SET_GROUP_INFORMATION_CFM",
	GW_GROUP_INFORMATION_CHANGED_NTF:           "GW_GROUP_INFORMATION_CHANGED_NTF",
	GW_DELETE_GROUP_REQ:                        "GW_DELETE_GROUP_REQ",
	GW_DELETE_GROUP_CFM:                        "GW_DELETE_GROUP_CFM",
	GW_NEW_GROUP_REQ:                           "GW_NEW_GROUP_REQ",
	GW_NEW_GROUP_CFM:                           "GW_NEW_GROUP_CFM",
	GW_GET_ALL_GROUPS_INFORMATION_REQ:          "GW_GET_ALL_GROUPS_INFORMATION_REQ",
	GW_GET_ALL_GROUPS_INFORMATION_CFM:          "GW_GET_ALL_GROUPS_INFORMATION_CFM",
	GW_GET_ALL_GROUPS_INFORMATION_NTF:          "GW_GET_ALL_GROUPS_INFORMATION_NTF",
	GW_GET_ALL_GROUPS_INFORMATION_FINISHED_NTF: "GW_GET_ALL_GROUPS_INFORMATION_FINISHED_NTF",
	GW_GROUP_DELETED_NTF:                       "GW_GROUP_DELETED_NTF",
	GW_HOUSE_STATUS_MONITOR_ENABLE_REQ:         "GW_HOUSE_STATUS_MONITOR_ENABLE_REQ",
	GW_HOUSE_STATUS_MONITOR_ENABLE_CFM:         "GW_HOUSE_STATUS_MONITOR_ENABLE_CFM",
	GW_HOUSE_STATUS_MONITOR_DISABLE_REQ:        "GW_HOUSE_STATUS_MONITOR_DISABLE_REQ",
	GW_HOUSE_STATUS_MONITOR_DISABLE_CFM:        "GW_HOUSE_STATUS_MONITOR_DISABLE_CFM",
	GW_COMMAND_SEND_REQ:                        "GW_COMMAND_SEND_REQ",
	GW_COMMAND_SEND_CFM:                        "GW_COMMAND_SEND_CFM",
	GW_COMMAND_RUN_STATUS_NTF:                  "GW_COMMAND_RUN_STATUS_NTF",
	GW_COMMAND_REMAINING_TIME_NTF:              "GW_COMMAND_REMAINING_TIME_NTF",
	GW_SESSION_FINISHED_NTF:                    "GW_SESSION_FINISHED_NTF",
	GW_STATUS_REQUEST_REQ:                      "GW_STATUS_REQUEST_REQ",
	GW_STATUS_REQUEST_CFM:                      "GW_STATUS_REQUEST_CFM",
	GW_STATUS_REQUEST_NTF:                      "GW_STATUS_REQUEST_NTF",
	GW_WINK_SEND_REQ:                           "GW_WINK_SEND_REQ",
	GW_WINK_SEND_CFM:                           "GW_WINK_SEND_CFM",
	GW_WINK_SEND_NTF:                           "GW_WINK_SEND_NTF",
	GW_SET_LIMITATION_REQ:                      "GW_SET_LIMITATION_REQ",
	GW_SET_LIMITATION_CFM:                      "GW_SET_LIMITATION_CFM",
	GW_GET_LIMITATION_STATUS_REQ:               "GW_GET_LIMITATION_STATUS_REQ",
	GW_GET_LIMITATION_STATUS_CFM:               "GW_GET_LIMITATION_STATUS_CFM",
	GW_LIMITATION_STATUS_NTF:                   "GW_LIMITATION_STATUS_NTF",
	GW_MODE_SEND_REQ:                           "GW_MODE_SEND_REQ",
	GW_MODE_SEND_CFM:                           "GW_MODE_SEND_CFM",
	GW_MODE_SEND_NTF:                           "GW_MODE_SEND_NTF",
	GW_INITIALIZE_SCENE_REQ:                    "GW_INITIALIZE_SCENE_REQ",
	GW_INITIALIZE_SCENE_CFM:                    "GW_INITIALIZE_SCENE_CFM",
	GW_INITIALIZE_SCENE_NTF:                    "GW_INITIALIZE_SCENE_NTF",
	GW_INITIALIZE_SCENE_CANCEL_REQ:             "GW_INITIALIZE_SCENE_CANCEL_REQ",
	GW_INITIALIZE_SCENE_CANCEL_CFM:             "GW_INITIALIZE_SCENE_CANCEL_CFM",
	GW_RECORD_SCENE_REQ:                        "GW_RECORD_SCENE_REQ",
	GW_RECORD_SCENE_CFM:                        "GW_RECORD_SCENE_CFM",
	GW_RECORD_SCENE_NTF:                        "GW_RECORD_SCENE_NTF",
	GW_DELETE_SCENE_REQ:                        "GW_DELETE_SCENE_REQ",
	GW_DELETE_SCENE_CFM:                        "GW_DELETE_SCENE_CFM",
	GW_RENAME_SCENE_REQ:                        "GW_RENAME_SCENE_REQ",
	GW_RENAME_SCENE_CFM:                        "GW_RENAME_SCENE_CFM",
	GW_GET_SCENE_LIST_REQ:                      "GW_GET_SCENE_LIST_REQ",
	GW_GET_SCENE_LIST_CFM:                      "GW_GET_SCENE_LIST_CFM",
	GW_GET_SCENE_LIST_NTF:                      "GW_GET_SCENE_LIST_NTF",
	GW_GET_SCENE_INFORMATION_REQ:               "GW_GET_SCENE_INFORMATION_REQ",
	GW_GET_SCENE_INFORMATION_CFM:               "GW_GET_SCENE_INFORMATION_CFM",
	GW_GET_SCENE_INFORMATION_NTF:               "GW_GET_SCENE_INFORMATION_NTF",
	GW_ACTIVATE_SCENE_REQ:                      "GW_ACTIVATE_SCENE_REQ",
	GW_ACTIVATE_SCENE_CFM:                      "GW_ACTIVATE_SCENE_CFM",
	GW_STOP_SCENE_REQ:                          "GW_STOP_SCENE_REQ",
	GW_STOP_SCENE_CFM:                          "GW_STOP_SCENE_CFM",
	GW_SCENE_INFORMATION_CHANGED_NTF:           "GW_SCENE_INFORMATION_CHANGED_NTF",
	GW_ACTIVATE_PRODUCTGROUP_REQ:               "GW_ACTIVATE_PRODUCTGROUP_REQ",
	GW_ACTIVATE_PRODUCTGROUP_CFM:               "GW_ACTIVATE_PRODUCTGROUP_CFM",
	GW_ACTIVATE_PRODUCTGROUP_NTF:               "GW_ACTIVATE_PRODUCTGROUP_NTF",
	GW_GET_CONTACT_INPUT_LINK_LIST_REQ:         "GW_GET_CONTACT_INPUT_LINK_LIST_REQ",
	GW_GET_CONTACT_INPUT_LINK_LIST_CFM:         "GW_GET_CONTACT_INPUT_LINK_LIST_CFM",
	GW_SET_CONTACT_INPUT_LINK_REQ:              "GW_SET_CONTACT_INPUT_LINK_REQ",
	GW_SET_CONTACT_INPUT_LINK_CFM:              "GW_SET_CONTACT_INPUT_LINK_CFM",
	GW_REMOVE_CONTACT_INPUT_LINK_REQ:           "GW_REMOVE_CONTACT_INPUT_LINK_REQ",
	GW_REMOVE_CONTACT_INPUT_LINK_CFM:           "GW_REMOVE_CONTACT_INPUT_LINK_CFM",
	GW_GET_ACTIVATION_LOG_HEADER_REQ:           "GW_GET_ACTIVATION_LOG_HEADER_REQ",
	GW_GET_ACTIVATION_LOG_HEADER_CFM:           "GW_GET_ACTIVATION_LOG_HEADER_CFM",
	GW_CLEAR_ACTIVATION_LOG_REQ:                "GW_CLEAR_ACTIVATION_LOG_REQ",
	GW_CLEAR_ACTIVATION_LOG_CFM:                "GW_CLEAR_ACTIVATION_LOG_CFM",
	GW_GET_ACTIVATION_LOG_LINE_REQ:             "GW_GET_ACTIVATION_LOG_LINE_REQ",
	GW_GET_ACTIVATION_LOG_LINE_CFM:             "GW_GET_ACTIVATION_LOG_LINE_CFM",
	GW_ACTIVATION_LOG_UPDATED_NTF:              "GW_ACTIVATION_LOG_UPDATED_NTF",
	GW_GET_MULTIPLE_ACTIVATION_LOG_LINES_REQ:   "GW_GET_MULTIPLE_ACTIVATION_LOG_LINES_REQ",
	GW_GET_MULTIPLE_ACTIVATION_LOG_LINES_NTF:   "GW_GET_MULTIPLE_ACTIVATION_LOG_LINES_NTF",
	GW_GET_MULTIPLE_ACTIVATION_LOG_LINES_CFN:   "GW_GET_MULTIPLE_ACTIVATION_LOG_LINES_CFN",
	GW_SET_UTC_REQ:                             "GW_SET_UTC_REQ",
	GW_SET_UTC_CFM:                             "GW_SET_UTC_CFM",
	GW_RTC_SET_TIME_ZONE_REQ:                   "GW_RTC_SET_TIME_ZONE_REQ",
	GW_RTC_SET_TIME_ZONE_CFM:                   "GW_RTC_SET_TIME_ZONE_CFM",
	GW_GET_LOCAL_TIME_REQ:                      "GW_GET_LOCAL_TIME_REQ",
	GW_GET_LOCAL_TIME_CFM:                      "GW_GET_LOCAL_TIME_CFM",
	GW_PASSWORD_ENTER_REQ:                      "GW_PASSWORD_ENTER_REQ",
	GW_PASSWORD_ENTER_CFM:                      "GW_PASSWORD_ENTER_CFM",
	GW_PASSWORD_CHANGE_REQ:                     "GW_PASSWORD_CHANGE_REQ",
	GW_PASSWORD_CHANGE_CFM:                     "GW_PASSWORD_CHANGE_CFM",
	GW_PASSWORD_CHANGE_NTF:                     "GW_PASSWORD_CHANGE_NTF",
}

// String returns the symbolic name of the command, or a hex fallback for
// unknown codes.
func (c Command) String() string {
	if name, ok := commandNames[c]; ok {
		return name
	}
	return "Command(0x" + strconv.FormatUint(uint64(c), 16) + ")"
}

// Originator identifies who caused an action on an actuator. Ported from const.Originator.
type Originator uint8

const (
	OriginatorUser                        Originator = 1
	OriginatorRain                        Originator = 2
	OriginatorTimer                       Originator = 3
	OriginatorUPS                         Originator = 5
	OriginatorSAAC                        Originator = 8
	OriginatorWind                        Originator = 9
	OriginatorLoadShedding                Originator = 11
	OriginatorLocalLight                  Originator = 12
	OriginatorUnspecificEnvironmentSensor Originator = 13
	OriginatorEmergency                   Originator = 255
)

// Priority for a command. Ported from const.Priority.
type Priority uint8

const (
	PriorityProtectionHuman       Priority = 0
	PriorityProtectionEnvironment Priority = 1
	PriorityUserLevel1            Priority = 2
	PriorityUserLevel2            Priority = 3
	PriorityComfortLevel1         Priority = 4
	PriorityComfortLevel2         Priority = 5
	PriorityComfortLevel3         Priority = 6
	PriorityComfortLevel4         Priority = 7
)

// LockPriorityLevel. Ported from const.LockPriorityLevel.
type LockPriorityLevel uint8

const (
	LockPriorityLevelNo      LockPriorityLevel = 0
	LockPriorityLevelMin30   LockPriorityLevel = 1
	LockPriorityLevelForever LockPriorityLevel = 2
)

// Velocity of a node movement. Ported from const.Velocity.
type Velocity uint8

const (
	VelocityDefault      Velocity = 0
	VelocitySilent       Velocity = 1
	VelocityFast         Velocity = 2
	VelocityNotAvailable Velocity = 255
)

// NodeTypeWithSubtype is a combined node type + subtype value.
// Ported from const.NodeTypeWithSubtype.
type NodeTypeWithSubtype uint16

const (
	NodeTypeWithSubtypeNoType                                      NodeTypeWithSubtype = 0
	NodeTypeWithSubtypeInteriorVenetianBlind                       NodeTypeWithSubtype = 0x0040
	NodeTypeWithSubtypeRollerShutter                               NodeTypeWithSubtype = 0x0080
	NodeTypeWithSubtypeAdjustableSlutsRollingShutter               NodeTypeWithSubtype = 0x0081
	NodeTypeWithSubtypeAdjustableSlutsRollingShutterWithProjection NodeTypeWithSubtype = 0x0082
	NodeTypeWithSubtypeVerticalExteriorAwning                      NodeTypeWithSubtype = 0x00C0
	NodeTypeWithSubtypeWindowOpener                                NodeTypeWithSubtype = 0x0100
	NodeTypeWithSubtypeWindowOpenerWithRainSensor                  NodeTypeWithSubtype = 0x0101
	NodeTypeWithSubtypeGarageDoorOpener                            NodeTypeWithSubtype = 0x0140
	NodeTypeWithSubtypeLinarAngularPositionOfGarageDoor            NodeTypeWithSubtype = 0x017A
	NodeTypeWithSubtypeLight                                       NodeTypeWithSubtype = 0x0180
	NodeTypeWithSubtypeLightOnOff                                  NodeTypeWithSubtype = 0x01BA
	NodeTypeWithSubtypeGateOpener                                  NodeTypeWithSubtype = 0x01C0
	NodeTypeWithSubtypeGateOpenerAngularPosition                   NodeTypeWithSubtype = 0x01FA
	NodeTypeWithSubtypeDoorLock                                    NodeTypeWithSubtype = 0x0240
	NodeTypeWithSubtypeWindowLock                                  NodeTypeWithSubtype = 0x0241
	NodeTypeWithSubtypeVerticalInteriorBlinds                      NodeTypeWithSubtype = 0x0280
	NodeTypeWithSubtypeDualRollerShutter                           NodeTypeWithSubtype = 0x0340
	NodeTypeWithSubtypeOnOffSwitch                                 NodeTypeWithSubtype = 0x03C0
	NodeTypeWithSubtypeHorizontalAwning                            NodeTypeWithSubtype = 0x0400
	NodeTypeWithSubtypeExteriorVenetianBlind                       NodeTypeWithSubtype = 0x0440
	NodeTypeWithSubtypeLouverBlind                                 NodeTypeWithSubtype = 0x0480
	NodeTypeWithSubtypeCurtainTrack                                NodeTypeWithSubtype = 0x04C0
	NodeTypeWithSubtypeVentilationPoint                            NodeTypeWithSubtype = 0x0500
	NodeTypeWithSubtypeVentilationPointAirInlet                    NodeTypeWithSubtype = 0x0502
	NodeTypeWithSubtypeVentilationPointAirTransfer                 NodeTypeWithSubtype = 0x0503
	// Note: pyvlx defines VENTILATION_POINT_AIR_OUTLET == 0x0503 (duplicate of AirTransfer).
	NodeTypeWithSubtypeVentilationPointAirOutlet            NodeTypeWithSubtype = 0x0503
	NodeTypeWithSubtypeExteriorHeating                      NodeTypeWithSubtype = 0x0540
	NodeTypeWithSubtypeSwingingShutters                     NodeTypeWithSubtype = 0x0600
	NodeTypeWithSubtypeSwingingShutterWithIndependentLeaves NodeTypeWithSubtype = 0x0601
	NodeTypeWithSubtypeBladeOpener                          NodeTypeWithSubtype = 0x0740
)

// NodeType. Ported from const.NodeType.
type NodeType uint8

const (
	NodeTypeNoType                      NodeType = 0
	NodeTypeVenetianBlind               NodeType = 1
	NodeTypeRollerShutter               NodeType = 2
	NodeTypeAwning                      NodeType = 3
	NodeTypeWindowOpener                NodeType = 4
	NodeTypeGarageOpener                NodeType = 5
	NodeTypeLight                       NodeType = 6
	NodeTypeGateOpener                  NodeType = 7
	NodeTypeRollingDoorOpener           NodeType = 8
	NodeTypeLock                        NodeType = 9
	NodeTypeBlind                       NodeType = 10
	NodeTypeSecureConfigurationDevice   NodeType = 11
	NodeTypeBeacon                      NodeType = 12
	NodeTypeDualShutter                 NodeType = 13
	NodeTypeHeatingTemperatureInterface NodeType = 14
	NodeTypeOnOffSwitch                 NodeType = 15
	NodeTypeHorizontalAwning            NodeType = 16
	NodeTypeExternalVenetianBlind       NodeType = 17
	NodeTypeLouvreBlint                 NodeType = 18
	NodeTypeCurtainTrack                NodeType = 19
	NodeTypeVentilationPoint            NodeType = 20
	NodeTypeExteriorHeating             NodeType = 21
	NodeTypeHeatPump                    NodeType = 22
	NodeTypeIntrusionAlarm              NodeType = 23
	NodeTypeSwingingShutter             NodeType = 24
)

// NodeVariation. Ported from const.NodeVariation.
type NodeVariation uint8

const (
	NodeVariationNotSet   NodeVariation = 0
	NodeVariationTophung  NodeVariation = 1
	NodeVariationKip      NodeVariation = 2
	NodeVariationFlatRoot NodeVariation = 3
	// Note: pyvlx defines SKY_LIGHT == 3 (duplicate of FLAT_ROOT).
	NodeVariationSkyLight NodeVariation = 3
)

// DHCPParameter. Ported from const.DHCPParameter.
type DHCPParameter uint8

const (
	DHCPParameterDisable DHCPParameter = 0x00
	DHCPParameterEnable  DHCPParameter = 0x01
)

// GatewayState. Ported from const.GatewayState.
type GatewayState uint8

const (
	GatewayStateTestMode                 GatewayState = 0
	GatewayStateGatewayModeNoActuator    GatewayState = 1
	GatewayStateGatewayModeWithActuators GatewayState = 2
	GatewayStateBeaconModeNotConfigured  GatewayState = 3
	GatewayStateBeaconModeConfigured     GatewayState = 4
)

// GatewaySubState. Ported from const.GatewaySubState.
type GatewaySubState uint8

const (
	GatewaySubStateIdle                                          GatewaySubState = 0x00
	GatewaySubStatePerformingTaskConfigurationServiceHandler     GatewaySubState = 0x01
	GatewaySubStatePerformingTaskSceneConfiguration              GatewaySubState = 0x02
	GatewaySubStatePerformingTaskInformationServiceConfiguration GatewaySubState = 0x03
	GatewaySubStatePerformingTaskContactInputConfiguration       GatewaySubState = 0x04
	GatewaySubStatePerformingTaskCommand                         GatewaySubState = 0x80
	GatewaySubStatePerformingTaskActivateGroup                   GatewaySubState = 0x81
	GatewaySubStatePerformingTaskActivateScene                   GatewaySubState = 0x82
	GatewaySubStateReserved132                                   GatewaySubState = 0x84
)

// LeaveLearnStateConfirmationStatus. Ported from const.LeaveLearnStateConfirmationStatus.
type LeaveLearnStateConfirmationStatus uint8

const (
	LeaveLearnStateConfirmationStatusFailed     LeaveLearnStateConfirmationStatus = 0
	LeaveLearnStateConfirmationStatusSuccessful LeaveLearnStateConfirmationStatus = 1
)

// ErrorNumber for GW_ERROR_NTF. Ported from const.ErrorNumber.
type ErrorNumber uint8

const (
	ErrorNumberUndefined        ErrorNumber = 0
	ErrorNumberWrongCommand     ErrorNumber = 1
	ErrorNumberFrameError       ErrorNumber = 2
	ErrorNumberBusy             ErrorNumber = 7
	ErrorNumberBadSystableIndex ErrorNumber = 8
	ErrorNumberNoAuth           ErrorNumber = 12
)

// ControllerCopyMode. Ported from const.ControllerCopyMode.
type ControllerCopyMode uint8

const (
	ControllerCopyModeTCM ControllerCopyMode = 0
	ControllerCopyModeRCM ControllerCopyMode = 1
)

// ControllerCopyStatus. Ported from const.ControllerCopyStatus.
type ControllerCopyStatus uint8

const (
	ControllerCopyStatusOK             ControllerCopyStatus = 0
	ControllerCopyStatusFailedTransfer ControllerCopyStatus = 1
	ControllerCopyStatusCancelled      ControllerCopyStatus = 4
	ControllerCopyStatusFailedTimeout  ControllerCopyStatus = 5
	ControllerCopyStatusFailedNotReady ControllerCopyStatus = 11
)

// ChangeKeyStatus. Ported from const.ChangeKeyStatus.
type ChangeKeyStatus uint8

const (
	ChangeKeyStatusOKController       ChangeKeyStatus = 0
	ChangeKeyStatusOKAll              ChangeKeyStatus = 2
	ChangeKeyStatusOKPartially        ChangeKeyStatus = 3
	ChangeKeyStatusOKReceived         ChangeKeyStatus = 5
	ChangeKeyStatusFailedNotDisabled  ChangeKeyStatus = 7
	ChangeKeyStatusFailedNoController ChangeKeyStatus = 9
	ChangeKeyStatusFailedDTSNotReady  ChangeKeyStatus = 10
	ChangeKeyStatusFailedDTSError     ChangeKeyStatus = 11
	ChangeKeyStatusFailedCSNotReady   ChangeKeyStatus = 16
)

// PgcJobState. Ported from const.PgcJobState.
type PgcJobState uint8

const (
	PgcJobStateStarted PgcJobState = 0
	PgcJobStateEnded   PgcJobState = 1
	PgcJobStateCSBusy  PgcJobState = 2
)

// PgcJobStatus. Ported from const.PgcJobStatus.
type PgcJobStatus uint8

const (
	PgcJobStatusOK          PgcJobStatus = 0
	PgcJobStatusOKPartially PgcJobStatus = 1
	PgcJobStatusFailedPGCCS PgcJobStatus = 2
	PgcJobStatusFailed      PgcJobStatus = 3
)

// PgcJobType. Ported from const.PgcJobType.
type PgcJobType uint8

const (
	PgcJobTypeReceiveOnly       PgcJobType = 0
	PgcJobTypeReceiveDistribute PgcJobType = 1
	PgcJobTypeTransmit          PgcJobType = 2
	PgcJobTypeGenerate          PgcJobType = 3
)

// DiscoverStatus. Ported from const.DiscoverStatus.
type DiscoverStatus uint8

const (
	DiscoverStatusOK               DiscoverStatus = 0
	DiscoverStatusFailedCSNotReady DiscoverStatus = 5
	DiscoverStatusOKPartially      DiscoverStatus = 6
	DiscoverStatusFailedCSBusy     DiscoverStatus = 7
)

// PowerMode. Ported from const.PowerMode.
type PowerMode uint8

const (
	PowerModeAlwaysAlive  PowerMode = 0
	PowerModeLowPowerMode PowerMode = 1
)

// ChangeType. Ported from const.ChangeType.
type ChangeType uint8

const (
	ChangeTypeDeleted  ChangeType = 0
	ChangeTypeModified ChangeType = 1
)

// ContactInputAssignement. Ported from const.ContactInputAssignement.
type ContactInputAssignement uint8

const (
	ContactInputAssignementNotAssinged  ContactInputAssignement = 0
	ContactInputAssignementScene        ContactInputAssignement = 1
	ContactInputAssignementProductGroup ContactInputAssignement = 2
	ContactInputAssignementByMode       ContactInputAssignement = 3
)

// OutputID. Ported from const.OutputID.
type OutputID uint8

const (
	OutputIDDontSend   OutputID = 0
	OutputIDPulsePort1 OutputID = 1
	OutputIDPulsePort2 OutputID = 2
	OutputIDPulsePort3 OutputID = 3
	OutputIDPulsePort4 OutputID = 4
	OutputIDPulsePort5 OutputID = 5
)

// GroupType. Ported from const.GroupType.
type GroupType uint8

const (
	GroupTypeUserGroup GroupType = 0
	GroupTypeRoom      GroupType = 1
	GroupTypeHouse     GroupType = 2
	GroupTypeAllGroup  GroupType = 3
)

// LimitationTimer. Ported from const.LimitationTimer.
type LimitationTimer uint8

const (
	LimitationTimerBySeconds   LimitationTimer = 1
	LimitationTimerUnlimited   LimitationTimer = 253
	LimitationTimerClearMaster LimitationTimer = 254
	LimitationTimerClearAll    LimitationTimer = 255
)

// LimitationType. Ported from const.LimitationType.
type LimitationType uint8

const (
	LimitationTypeMinLimitation LimitationType = 0
	LimitationTypeMaxLimitation LimitationType = 1
)

// LockTime. Ported from const.LockTime.
type LockTime uint8

const (
	LockTimeBySeconds LockTime = 1
	LockTimeUnlimited LockTime = 255
)

// WinkTime. Ported from const.WinkTime.
type WinkTime uint8

const (
	WinkTimeStop            WinkTime = 0
	WinkTimeBySeconds       WinkTime = 1
	WinkTimeByManufactuerer WinkTime = 254
	WinkTimeForever         WinkTime = 255
)

// NodeParameter. Ported from const.NodeParameter.
type NodeParameter uint8

const (
	NodeParameterMP      NodeParameter = 0x00
	NodeParameterFP1     NodeParameter = 0x01
	NodeParameterFP2     NodeParameter = 0x02
	NodeParameterFP3     NodeParameter = 0x03
	NodeParameterFP4     NodeParameter = 0x04
	NodeParameterFP5     NodeParameter = 0x05
	NodeParameterFP6     NodeParameter = 0x06
	NodeParameterFP7     NodeParameter = 0x07
	NodeParameterFP8     NodeParameter = 0x08
	NodeParameterFP9     NodeParameter = 0x09
	NodeParameterFP10    NodeParameter = 0x0A
	NodeParameterFP11    NodeParameter = 0x0B
	NodeParameterFP12    NodeParameter = 0x0C
	NodeParameterFP13    NodeParameter = 0x0D
	NodeParameterFP14    NodeParameter = 0x0E
	NodeParameterFP15    NodeParameter = 0x0F
	NodeParameterFP16    NodeParameter = 0x10
	NodeParameterNotUsed NodeParameter = 0xFF
)

// OperatingState. Ported from const.OperatingState.
type OperatingState uint8

const (
	OperatingStateNonExecuting   OperatingState = 0
	OperatingStateErrorExecuting OperatingState = 1
	OperatingStateNotUsed        OperatingState = 2
	OperatingStateWaitForPower   OperatingState = 3
	OperatingStateExecuting      OperatingState = 4
	OperatingStateDone           OperatingState = 5
	OperatingStateUnknown        OperatingState = 255
)

// StatusReply. Ported from const.StatusReply.
type StatusReply uint8

const (
	StatusReplyUnknownStatusReply                  StatusReply = 0x00
	StatusReplyCommandCompletedOK                  StatusReply = 0x01
	StatusReplyNoContact                           StatusReply = 0x02
	StatusReplyManuallyOperated                    StatusReply = 0x03
	StatusReplyBlocked                             StatusReply = 0x04
	StatusReplyWrongSystemkey                      StatusReply = 0x05
	StatusReplyPriorityLevelLocked                 StatusReply = 0x06
	StatusReplyReachedWrongPosition                StatusReply = 0x07
	StatusReplyErrorDuringExecution                StatusReply = 0x08
	StatusReplyNoExecution                         StatusReply = 0x09
	StatusReplyCalibrating                         StatusReply = 0x0A
	StatusReplyPowerConsumptionTooHigh             StatusReply = 0x0B
	StatusReplyPowerConsumptionTooLow              StatusReply = 0x0C
	StatusReplyLockPositionOpen                    StatusReply = 0x0D
	StatusReplyMotionTimeTooLongCommunicationEnded StatusReply = 0x0E
	StatusReplyThermalProtection                   StatusReply = 0x0F
	StatusReplyProductNotOperational               StatusReply = 0x10
	StatusReplyFilterMaintenanceNeeded             StatusReply = 0x11
	StatusReplyBatteryLevel                        StatusReply = 0x12
	StatusReplyTargetModified                      StatusReply = 0x13
	StatusReplyModeNotImplemented                  StatusReply = 0x14
	StatusReplyCommandIncompatibleToMovement       StatusReply = 0x15
	StatusReplyUserAction                          StatusReply = 0x16
	StatusReplyDeadBoltError                       StatusReply = 0x17
	StatusReplyAutomaticCycleEngaged               StatusReply = 0x18
	StatusReplyWrongLoadConnected                  StatusReply = 0x19
	StatusReplyColourNotReachable                  StatusReply = 0x1A
	StatusReplyTargetNotReachable                  StatusReply = 0x1B
	StatusReplyBadIndexReceived                    StatusReply = 0x1C
	StatusReplyCommandOverruled                    StatusReply = 0x1D
	StatusReplyNodeWaitingForPower                 StatusReply = 0x1E
	StatusReplyInformationCode                     StatusReply = 0xDF
	StatusReplyParameterLimited                    StatusReply = 0xE0
	StatusReplyLimitationByLocalUser               StatusReply = 0xE1
	StatusReplyLimitationByUser                    StatusReply = 0xE2
	StatusReplyLimitationByRain                    StatusReply = 0xE3
	StatusReplyLimitationByTimer                   StatusReply = 0xE4
	StatusReplyLimitationByUPS                     StatusReply = 0xE6
	StatusReplyLimitationByUnknownDevice           StatusReply = 0xE7
	StatusReplyLimitationBySAAC                    StatusReply = 0xEA
	StatusReplyLimitationByWind                    StatusReply = 0xEB
	StatusReplyLimitationByMyself                  StatusReply = 0xEC
	StatusReplyLimitationByAutomaticCycle          StatusReply = 0xED
	StatusReplyLimitationByEmergency               StatusReply = 0xEE
)

// StatusId. Ported from const.StatusId.
type StatusId uint8

const (
	StatusIdStatusUser           StatusId = 0x01
	StatusIdStatusRain           StatusId = 0x02
	StatusIdStatusTimer          StatusId = 0x03
	StatusIdStatusUPS            StatusId = 0x05
	StatusIdStatusProgram        StatusId = 0x08
	StatusIdStatusWind           StatusId = 0x09
	StatusIdStatusMyself         StatusId = 0x0A
	StatusIdStatusAutomaticCycle StatusId = 0x0B
	StatusIdStatusEmergency      StatusId = 0x0C
	StatusIdStatusUnknown        StatusId = 0xFF
)

// RunStatus. Ported from const.RunStatus.
type RunStatus uint8

const (
	RunStatusExecutionCompleted RunStatus = 0
	RunStatusExecutionFailed    RunStatus = 1
	RunStatusExecutionActive    RunStatus = 2
)

// StatusType for GW_STATUS_REQUEST_NTF. Ported from const.StatusType.
type StatusType uint8

const (
	StatusTypeRequestTargetPosition  StatusType = 0
	StatusTypeRequestCurrentPosition StatusType = 1
	StatusTypeRequestRemainingTime   StatusType = 2
	StatusTypeRequestMainInfo        StatusType = 3
)
