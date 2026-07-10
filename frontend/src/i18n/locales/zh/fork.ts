// Fork-specific i18n keys (imageStudio, videoStudio, home, and fork additions to
// admin/nav). Kept SEPARATE from the upstream modular locale files so upstream
// syncs stay conflict-free; deep-merged over the upstream base in ./index.ts.
export default {
  "home": {
    "navSubtitle": "面向 AI 创作团队的 API",
    "heroEyebrow": "Sub2API for code, image and model routing",
    "heroTitle": "一把 API，连接 AI 编码、图像生成与模型调度",
    "values": {
      "oneKey": {
        "title": "一把密钥",
        "description": "把多个模型供应商抽象成统一 API，减少账号、端点和凭证切换。"
      },
      "routing": {
        "title": "稳定路由",
        "description": "按模型、账号池和限流状态调度请求，让代码和图像任务更可持续。"
      },
      "cost": {
        "title": "成本清晰",
        "description": "额度、用量、账单和倍率统一可见，适合个人创作和小团队协作。"
      }
    },
    "product": {
      "eyebrow": "Product system",
      "title": "不是另一个壳，而是一层 AI 基础设施",
      "description": "Sub2API 把模型接入、调度、计费和用量治理放在同一个产品里。首页负责说明能力，控制台负责把这些能力变成日常工作流。"
    },
    "workflow": {
      "eyebrow": "Workflow",
      "title": "代码与图像，用同一套接口治理",
      "description": "同一把 API Key 可以进入编码、图片、用量统计和后台调度。视觉上不需要假装复杂，产品本身要足够清楚。",
      "codeTitle": "Code request",
      "imageTitle": "Image request",
      "prompt": "Prompt",
      "promptValue": "极简 AI 产品界面，清晰的输入、模型选择、任务状态与用量记录。",
      "model": "Model",
      "modelValue": "按任务路由到 Claude、GPT、Gemini 或图像模型。",
      "policy": "Control",
      "policyValue": "额度、倍率、账号池和失败切换由平台统一执行。"
    },
    "showcase": {
      "title": "AI 创作工作台",
      "code": {
        "title": "AI 编写代码",
        "subtitle": "从需求、重构到调试，统一走同一把密钥。",
        "line1": "POST /v1/messages  # 生成 Vue 组件",
        "line2": "route.model = claude / gpt / gemini",
        "line3": "stream.done  代码、说明、测试建议已返回"
      },
      "image": {
        "title": "AI 生成图片",
        "subtitle": "用提示词、尺寸和风格参数描述画面，而不是依赖静态素材。",
        "promptLabel": "Prompt",
        "prompt": "一个暖白色 AI 工作台界面，代码窗口与图像提示词并排，青绿色状态线条，琥珀色重点操作。"
      },
      "palette": {
        "ink": "Ink",
        "clay": "Clay",
        "mint": "Mint",
        "sky": "Sky"
      }
    },
    "metrics": {
      "key": "统一密钥",
      "models": "模型路由",
      "workflow": "创作工作流"
    }
  },
  "nav": {
    "recharge": "充值",
    "imageStudio": "图像生成",
    "videoStudio": "视频生成"
  },
  "videoStudio": {
    "title": "视频生成",
    "workbenchTitle": "视频生成工作台",
    "workbenchSubtitle": "输入描述，几分钟后获得 AI 生成的视频。",
    "group": "分组",
    "model": "模型",
    "prompt": "提示词",
    "promptPlaceholder": "描述你想生成的视频画面、动作和风格…",
    "estimatedCost": "预估费用",
    "balance": "余额",
    "generate": "生成视频",
    "retentionNotice": "视频为短期有效，生成后请及时下载保存。",
    "myVideos": "我的视频",
    "refresh": "刷新状态",
    "clearHistory": "清空历史",
    "emptyTitle": "还没有视频",
    "emptySubtitle": "在左侧输入提示词，开始生成你的第一个视频。",
    "deleteTitle": "删除视频",
    "deleteMessage": "确定删除这条视频生成记录吗？此操作不可恢复。",
    "clearHistoryTitle": "清空视频历史",
    "clearHistoryMessage": "确定清空全部视频生成记录吗？此操作不可恢复。",
    "play": "播放",
    "download": "下载",
    "retry": "重试",
    "loadingVideo": "正在加载视频…",
    "videoExpired": "视频链接已过期，无法播放或下载。",
    "statusProcessing": "生成中…",
    "processingHint": "Veo 视频生成通常需要数分钟，请耐心等待。",
    "error": {
      "no_account": "当前分组没有可用的视频账号。",
      "submit_failed": "提交生成请求失败，请稍后重试。",
      "upstream_error": "上游生成失败，请重试或更换提示词。",
      "interrupted": "生成被中断，请重试。",
      "generic": "生成失败，请重试。"
    }
  },
  "imageStudio": {
    "workbenchSubtitle": "在一个工作台里完成生成、编辑和多图组图。",
    "capabilityGenerate": "生成",
    "capabilityEdit": "编辑",
    "capabilityCompose": "组图",
    "capabilityHistory": "历史",
    "capabilityGenerateCopy": "从提示词一次生成一张或多张图。",
    "capabilityEditCopy": "上传参考图，用于编辑或多图组图。",
    "capabilityHistoryCopy": "回看、删除、清空，超时任务也能继续等待。",
    "stepCreate": "创作",
    "stepReference": "参考图",
    "stepPrompt": "提示词",
    "workbenchModeTitle": "模式与输出",
    "referenceWorkbenchTitle": "参考图工作区",
    "promptTitle": "描述你想要的结果",
    "title": "绘图",
    "workbenchTitle": "在线画图工作台",
    "subtitle": "使用 AI 生成图片",
    "conversations": "会话列表",
    "newConversation": "新建会话",
    "noConversations": "暂无会话",
    "allGenerations": "全部图片",
    "emptyGalleryTitle": "还没有图片",
    "emptyGalleryDescription": "开启一个新会话来生成你的第一张图片",
    "emptyTurnsTitle": "开始生成",
    "emptyTurnsDescription": "在下方输入描述词来生成图片",
    "promptPlaceholder": "描述你想要生成的图片… 例如“雨天窗边温馨的阅读角落，暖色灯光，水彩风格”",
    "generating": "生成中…",
    "retry": "重试",
    "model": "模型",
    "group": "分组",
    "selectGroup": "选择分组",
    "noImageGroupHint": "未找到支持图片生成的分组，请联系管理员。",
    "size": "尺寸",
    "quality": "质量",
    "qualityAuto": "自动",
    "qualityHigh": "高",
    "qualityMedium": "中",
    "qualityLow": "低",
    "qualityStandard": "标准",
    "qualityHd": "高清",
    "onboardingTitle": "让灵感成为画面",
    "onboardingSubtitle": "在同一处保留历史与任务状态，并从任意结果继续创作。",
    "balanceShort": "剩余",
    "sendAria": "生成图片",
    "examplePrompt1": "清晨的日式庭院，锦鲤池塘，薄雾轻笼",
    "examplePrompt2": "复古未来主义城市天际线，日落时分，霓虹倒影，电影感",
    "examplePrompt3": "一个友好的机器人咖啡师在制作咖啡，温馨的咖啡馆，暖色灯光",
    "examplePrompt4": "花海中狐狸的水彩肖像，梦幻的粉彩色调",
    "qualityChip": "质量：{quality}",
    "countChip": "数量：{count}",
    "count": "数量",
    "cost": "消耗",
    "estimatedCost": "预计消耗",
    "untitled": "未命名",
    "noImages": "暂无图片",
    "generationFailed": "生成失败",
    "errorCodes": {
      "no_account": "暂无可用的生成账号，请稍后重试。",
      "no_images": "上游未返回图片，请重试。",
      "content_blocked": "内容未通过安全审核，请调整提示词或参考图后重试。",
      "interrupted": "生成被中断（服务可能已重启），请重试。",
      "upstream_error": "生成失败，请重试。",
      "busy": "生成请求过于频繁，请稍后重试。",
      "storage_error": "图片保存失败，请重试。"
    },
    "imageLoadFailed": "图片加载失败",
    "imageLoading": "图片加载中",
    "cachedUrlFallback": "缓存链接",
    "deleteConversationTitle": "删除会话？",
    "deleteConversationMessage": "该操作将永久删除此会话及其所有图片。",
    "deleteGenerationTitle": "删除图片？",
    "deleteGenerationMessage": "该操作将永久删除此生成图片。",
    "conversationDeleted": "会话已删除",
    "generationDeleted": "图片已删除",
    "errorGeneric": "发生错误，请重试。",
    "retryReferenceFetchFailed": "参考图获取失败，本次重试将作为文生图进行。",
    "errorGroupNotEnabled": "当前分组未开启图片生成功能。",
    "continueWaitingHint": "任务已保存在历史中，可以离开后回来继续等待，工作台会持续轮询状态。",
    "waitingElapsed": "已等待 {elapsed}",
    "loadEarlier": "加载更早图片",
    "loadingEarlier": "加载中…",
    "refreshStatus": "刷新状态",
    "statusRefreshed": "生成状态已刷新",
    "imageSettings": "图像设置",
    "aspectRatio": "宽高比",
    "customSize": "自定义尺寸",
    "width": "宽",
    "height": "高",
    "aspectAuto": "自动",
    "upload": "上传",
    "mode": "模式",
    "modeGenerate": "生成",
    "modeEdit": "编辑",
    "modeCompose": "组图",
    "modeGenerateHint": "仅使用文字提示，不发送参考图。",
    "modeEditHint": "上传 1 张参考图，用于编辑或改风格。",
    "modeComposeHint": "上传 2 张及以上参考图，用于多图组图。",
    "referenceRequirementGenerate": "0 张参考图",
    "referenceRequirementEdit": "需要 1 张参考图",
    "referenceRequirementCompose": "需要 2+ 张参考图",
    "referenceImage": "参考图",
    "sourceImage": "原图",
    "imageToImage": "图生图",
    "removeReference": "移除",
    "imageTypeError": "请选择图片文件",
    "imageTooLarge": "图片过大(上限 20MB)",
    "tooManyReferences": "最多支持 {count} 张参考图",
    "clearHistory": "清空",
    "clearHistoryTitle": "清空图片历史？",
    "clearHistoryMessage": "此操作将永久删除全部作图会话和图片。",
    "historyCleared": "图片历史已清空",
    "countShort": "{count}张",
    "modelGroupImage": "图像模型",
    "modelGroupRouting": "自动路由",
    "modelGroupGpt5": "GPT-5 系列",
    "quickEdit": "编辑",
    "quickEditReady": "已将这张图加入参考图，请描述想要的编辑效果。",
    "addReference": "加入参考",
    "referenceAdded": "已加入参考图。",
    "download": "下载",
    "downloadImage": "下载图片",
    "downloadStarted": "已开始下载。",
    "downloadFailed": "下载失败，请稍后重试。",
    "retentionNoticeTitle": "图片仅在服务器保留 24 小时",
    "retentionNoticeBody": "请及时下载需要保留的图片；服务器缓存和历史预览会在 24 小时后自动清理。",
    "selectFromHistory": "从历史选择",
    "noHistoryImages": "暂无历史图片",
    "conversationHistory": "历史会话"
  },
  "admin": {
    "groups": {
      "soraPricing": {
        "title": "Sora 按次计费",
        "description": "配置 Sora 图片/视频按次收费价格，留空则默认不计费",
        "image360": "图片 360px ($)",
        "image540": "图片 540px ($)",
        "video": "视频（标准）($)",
        "videoHd": "视频（Pro-HD）($)",
        "storageQuota": "存储配额",
        "storageQuotaHint": "单位 GB，设置该分组用户的 Sora 存储配额上限，0 表示使用系统默认"
      },
      "veoPricing": {
        "title": "Veo 视频计费（按秒）",
        "description": "配置 Veo 视频生成的每秒单价，留空则默认不计费",
        "videoPerSecond": "视频单价（每秒 $）"
      },
      "claudeMaxSimulation": {
        "title": "Claude Max 用量模拟",
        "tooltip": "开启后，对于上游没有缓存写入用量的 Claude 模型，系统会确定性地映射为少量输入和 1 小时缓存创建，同时保持总 token 不变。",
        "enabled": "已启用（模拟 1 小时缓存）",
        "disabled": "未启用",
        "hint": "仅调整用量计费日志中的 token 类别，不会持久化每次请求的映射状态。"
      }
    },
    "channels": {
      "form": {
        "syncGroupSupportedModels": "同步分组账号支持的模型",
        "syncingGroupModels": "同步账号模型中...",
        "syncGroupModelsNoGroups": "请先选择关联分组",
        "syncGroupModelsNoAccounts": "已选分组下没有账号，请先把账号绑定到该分组",
        "syncGroupModelsNoModels": "已选分组下 {accounts} 个账号没有可同步的模型，请检查账号模型映射或账号类型",
        "syncGroupModelsSuccess": "已从 {accounts} 个账号同步 {count} 个模型，其中 {priced} 个已填入官方价格",
        "syncGroupModelsAlreadyUpToDate": "分组账号模型已全部在定价规则中",
        "syncGroupModelsError": "同步分组账号模型失败"
      }
    },
    "accounts": {
      "openai": {
        "codexCLIOnlyAllowClaudeCode": "额外放行 Claude Code 的 Codex 插件",
        "codexCLIOnlyAllowClaudeCodeDesc": "仅在上方开关开启时生效。额外放行通过 Claude Code 的 Codex 插件发起的请求（精确匹配 originator=Claude Code），不影响对其他非官方客户端的拦截。",
        "codexImageGenerationBridge": "Codex 图片生成桥接",
        "codexImageGenerationBridgeDesc": "账号级策略优先于渠道和全局配置。仅控制 Codex 走 /responses 文本端点时是否注入 image_generation 工具；不影响独立图片生成接口。",
        "codexImageGenerationBridgeInherit": "跟随渠道",
        "codexImageGenerationBridgeInheritDesc": "不写入账号覆盖，继续使用渠道或全局策略。",
        "codexImageGenerationBridgeEnabled": "强制开启",
        "codexImageGenerationBridgeEnabledDesc": "允许 Codex /responses 请求获得图片工具注入。",
        "codexImageGenerationBridgeDisabled": "强制关闭",
        "codexImageGenerationBridgeDisabledDesc": "阻断 Codex /responses 的图片工具注入。",
        "codexImageGenerationBridgeBadgeInherit": "渠道策略",
        "codexImageGenerationBridgeBadgeEnabled": "账号开启",
        "codexImageGenerationBridgeBadgeDisabled": "账号关闭"
      }
    },
    "settings": {
      "gatewayForwarding": {
        "openaiAllowClaudeCodeCodexPlugin": "允许在 Claude Code 中使用 Codex 插件",
        "openaiAllowClaudeCodeCodexPluginDesc": "全局开关，仅对已开启「仅允许 Codex 官方客户端」的 OpenAI OAuth 账号生效。开启后，所有此类账号都额外放行通过 Claude Code 的 Codex 插件发起的请求（精确匹配 originator=Claude Code），无需逐账号配置；上游请求仍保持透传。"
      },
      "site": {
        "externalRechargeEnabled": "启用外部充值入口",
        "externalRechargeEnabledHint": "启用后,余额数字、顶栏\"充值\"按钮与侧边栏\"充值/订阅\"将跳转到下方外部地址(新标签打开);未配置地址时回退到站内充值页。",
        "externalRechargeUrl": "外部充值地址",
        "externalRechargeUrlHint": "点击充值时在新标签打开的外部链接,需以 http:// 或 https:// 开头。留空则使用站内充值。",
        "externalRechargeUrlPlaceholder": "https://your-recharge-site.com"
      }
    }
  }
} as const
