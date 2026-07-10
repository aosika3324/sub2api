// Fork-specific i18n keys (imageStudio, videoStudio, home, and fork additions to
// admin/nav). Kept SEPARATE from the upstream modular locale files so upstream
// syncs stay conflict-free; deep-merged over the upstream base in ./index.ts.
export default {
  "home": {
    "navSubtitle": "API for creative AI teams",
    "heroEyebrow": "Sub2API for code, image, and model routing",
    "heroTitle": "One API for AI coding, image generation, and model routing",
    "values": {
      "oneKey": {
        "title": "One key",
        "description": "Abstract multiple model providers behind one API and reduce account, endpoint, and credential switching."
      },
      "routing": {
        "title": "Reliable routing",
        "description": "Schedule requests by model, account pool, and rate-limit state so code and image jobs keep moving."
      },
      "cost": {
        "title": "Clear cost",
        "description": "Quota, usage, billing, and multipliers stay visible for solo creators and small teams."
      }
    },
    "product": {
      "eyebrow": "Product system",
      "title": "Not another wrapper. A layer for AI infrastructure.",
      "description": "Sub2API brings model access, routing, billing, and usage governance into one product. The landing page explains the system; the dashboard turns it into daily workflow."
    },
    "workflow": {
      "eyebrow": "Workflow",
      "title": "Code and images governed by the same interface",
      "description": "The same API key can power coding, image work, usage analytics, and backend routing. The interface can stay quiet because the product is clear.",
      "codeTitle": "Code request",
      "imageTitle": "Image request",
      "prompt": "Prompt",
      "promptValue": "A minimal AI product interface with clear input, model choice, job state, and usage records.",
      "model": "Model",
      "modelValue": "Route by task to Claude, GPT, Gemini, or an image model.",
      "policy": "Control",
      "policyValue": "Quota, multipliers, account pools, and failover are enforced by the platform."
    },
    "showcase": {
      "title": "AI creation workspace",
      "code": {
        "title": "AI writes code",
        "subtitle": "From specs and refactors to debugging, every request uses the same key.",
        "line1": "POST /v1/messages  # generate Vue component",
        "line2": "route.model = claude / gpt / gemini",
        "line3": "stream.done  code, notes, tests returned"
      },
      "image": {
        "title": "AI creates images",
        "subtitle": "Describe the frame with prompt, size, and style controls instead of static assets.",
        "promptLabel": "Prompt",
        "prompt": "A warm off-white AI workspace with code and image prompts side by side, mint status lines, and clay action accents."
      },
      "palette": {
        "ink": "Ink",
        "clay": "Clay",
        "mint": "Mint",
        "sky": "Sky"
      }
    },
    "metrics": {
      "key": "Unified key",
      "models": "Model routing",
      "workflow": "Creation workflow"
    }
  },
  "nav": {
    "recharge": "Recharge",
    "imageStudio": "Image Studio"
  },
  "videoStudio": {
    "title": "Video Studio",
    "workbenchTitle": "Video generation workbench",
    "workbenchSubtitle": "Describe a scene and get an AI-generated video in a few minutes.",
    "group": "Group",
    "model": "Model",
    "prompt": "Prompt",
    "promptPlaceholder": "Describe the scene, action, and style you want to generate…",
    "estimatedCost": "Estimated cost",
    "balance": "Balance",
    "generate": "Generate video",
    "retentionNotice": "Videos are available for a limited time — download them promptly after generation.",
    "myVideos": "My videos",
    "refresh": "Refresh status",
    "clearHistory": "Clear history",
    "emptyTitle": "No videos yet",
    "emptySubtitle": "Enter a prompt on the left to generate your first video.",
    "deleteTitle": "Delete video",
    "deleteMessage": "Delete this video generation? This cannot be undone.",
    "clearHistoryTitle": "Clear video history",
    "clearHistoryMessage": "Clear all video generations? This cannot be undone.",
    "play": "Play",
    "download": "Download",
    "retry": "Retry",
    "loadingVideo": "Loading video…",
    "videoExpired": "The video link has expired and can no longer be played or downloaded.",
    "statusProcessing": "Generating…",
    "processingHint": "Veo video generation usually takes a few minutes. Please wait.",
    "error": {
      "no_account": "No video account is available for this group.",
      "submit_failed": "Failed to submit the generation request. Please try again later.",
      "upstream_error": "Upstream generation failed. Please retry or adjust your prompt.",
      "interrupted": "Generation was interrupted. Please retry.",
      "generic": "Generation failed. Please try again."
    }
  },
  "imageStudio": {
    "title": "Image Studio",
    "workbenchTitle": "Online drawing workbench",
    "workbenchSubtitle": "Generate, edit, and compose multiple reference images in one place.",
    "capabilityGenerate": "Generate",
    "capabilityEdit": "Edit",
    "capabilityCompose": "Compose",
    "capabilityHistory": "History",
    "capabilityGenerateCopy": "Create one or many images from a prompt.",
    "capabilityEditCopy": "Upload references for edit and composition modes.",
    "capabilityHistoryCopy": "Review, delete, clear, and keep waiting for saved jobs.",
    "stepCreate": "Create",
    "stepReference": "References",
    "stepPrompt": "Prompt",
    "workbenchModeTitle": "Mode and output",
    "referenceWorkbenchTitle": "Reference image tray",
    "promptTitle": "Describe the result",
    "subtitle": "Generate images with AI",
    "conversations": "Conversations",
    "newConversation": "New Conversation",
    "noConversations": "No conversations yet",
    "allGenerations": "All Generations",
    "emptyGalleryTitle": "No images yet",
    "emptyGalleryDescription": "Start a new conversation to generate your first image",
    "emptyTurnsTitle": "Start generating",
    "emptyTurnsDescription": "Type a prompt below to generate an image",
    "promptPlaceholder": "Describe the image you want to create… e.g. “a cozy reading nook by a rainy window, warm lamplight, watercolor”",
    "generating": "Generating…",
    "retry": "Retry",
    "model": "Model",
    "group": "Group",
    "selectGroup": "Select a group",
    "noImageGroupHint": "No image-capable group found. Please contact your administrator.",
    "size": "Size",
    "quality": "Quality",
    "qualityAuto": "Auto",
    "qualityHigh": "High",
    "qualityMedium": "Medium",
    "qualityLow": "Low",
    "qualityStandard": "Standard",
    "qualityHd": "HD",
    "onboardingTitle": "Turn ideas into images",
    "onboardingSubtitle": "Keep your history and tasks in one place, and pick up from any result.",
    "balanceShort": "Left",
    "sendAria": "Generate image",
    "examplePrompt1": "A serene Japanese garden at dawn, koi pond, soft morning mist",
    "examplePrompt2": "Retro-futuristic city skyline at sunset, neon reflections, cinematic",
    "examplePrompt3": "A friendly robot barista making coffee, cozy cafe, warm lighting",
    "examplePrompt4": "Watercolor portrait of a fox in a flower meadow, dreamy pastels",
    "qualityChip": "Quality: {quality}",
    "countChip": "Count: {count}",
    "count": "Count",
    "cost": "Cost",
    "estimatedCost": "Estimated cost",
    "untitled": "Untitled",
    "noImages": "No images",
    "generationFailed": "Generation failed",
    "errorCodes": {
      "no_account": "No generation account is available right now. Please try again shortly.",
      "no_images": "The upstream returned no images. Please retry.",
      "content_blocked": "Content was blocked by moderation. Adjust the prompt or reference image and retry.",
      "interrupted": "Generation was interrupted (the service may have restarted). Please retry.",
      "upstream_error": "Generation failed. Please retry.",
      "busy": "Too many generation requests. Please try again shortly.",
      "storage_error": "Failed to save the image. Please retry."
    },
    "imageLoadFailed": "Image failed to load",
    "imageLoading": "Loading image",
    "cachedUrlFallback": "Cached URL",
    "deleteConversationTitle": "Delete conversation?",
    "deleteConversationMessage": "This will permanently delete this conversation and all its images.",
    "deleteGenerationTitle": "Delete image?",
    "deleteGenerationMessage": "This will permanently delete this generated image.",
    "conversationDeleted": "Conversation deleted",
    "generationDeleted": "Image deleted",
    "errorGeneric": "An error occurred. Please try again.",
    "retryReferenceFetchFailed": "Could not load the reference image; this retry will run as text-to-image.",
    "errorGroupNotEnabled": "Image generation is not enabled for this group.",
    "continueWaitingHint": "This job is saved in history. You can leave and come back; the studio will keep polling it.",
    "waitingElapsed": "Waiting {elapsed}",
    "loadEarlier": "Load earlier images",
    "loadingEarlier": "Loading…",
    "refreshStatus": "Refresh status",
    "statusRefreshed": "Generation status refreshed",
    "imageSettings": "Image settings",
    "aspectRatio": "Aspect ratio",
    "customSize": "Custom size",
    "width": "Width",
    "height": "Height",
    "aspectAuto": "Auto",
    "upload": "Upload",
    "mode": "Mode",
    "modeGenerate": "Generate",
    "modeEdit": "Edit",
    "modeCompose": "Compose",
    "modeGenerateHint": "Text prompt only. No reference image is sent.",
    "modeEditHint": "Use one reference image to edit or restyle an existing image.",
    "modeComposeHint": "Use two or more reference images for composition.",
    "referenceRequirementGenerate": "0 refs",
    "referenceRequirementEdit": "1 ref required",
    "referenceRequirementCompose": "2+ refs required",
    "referenceImage": "Reference",
    "sourceImage": "Source",
    "imageToImage": "Image to image",
    "removeReference": "Remove",
    "imageTypeError": "Please select an image file",
    "imageTooLarge": "Image is too large (max 20MB)",
    "tooManyReferences": "Up to {count} reference images",
    "clearHistory": "Clear",
    "clearHistoryTitle": "Clear image history?",
    "clearHistoryMessage": "This will permanently delete all image conversations and images.",
    "historyCleared": "Image history cleared",
    "countShort": "{count} imgs",
    "modelGroupImage": "Image models",
    "modelGroupRouting": "Routing",
    "modelGroupGpt5": "GPT-5 family",
    "quickEdit": "Edit",
    "quickEditReady": "Image added as a reference. Describe the edit you want.",
    "addReference": "Add ref",
    "referenceAdded": "Image added to references.",
    "download": "Download",
    "downloadImage": "Download image",
    "downloadStarted": "Download started.",
    "downloadFailed": "Download failed. Please try again.",
    "retentionNoticeTitle": "Images are kept for 24 hours",
    "retentionNoticeBody": "Download any image you want to keep. Server copies and history previews are automatically removed after 24 hours.",
    "selectFromHistory": "Choose from history",
    "noHistoryImages": "No history images yet",
    "conversationHistory": "History"
  },
  "admin": {
    "groups": {
      "soraPricing": {
        "title": "Sora Per-Request Pricing",
        "description": "Configure per-request pricing for Sora image/video generation. Leave empty to disable billing.",
        "image360": "Image 360px ($)",
        "image540": "Image 540px ($)",
        "video": "Video (standard) ($)",
        "videoHd": "Video (Pro-HD) ($)",
        "storageQuota": "Storage Quota",
        "storageQuotaHint": "In GB, set the Sora storage quota for users in this group. 0 means use system default"
      }
    },
    "channels": {
      "form": {
        "syncGroupSupportedModels": "Sync Group Account Models",
        "syncingGroupModels": "Syncing account models...",
        "syncGroupModelsNoGroups": "Select linked groups first",
        "syncGroupModelsNoAccounts": "No accounts are linked to the selected groups",
        "syncGroupModelsNoModels": "The {accounts} account(s) in the selected groups have no syncable models. Check account model mappings or account types.",
        "syncGroupModelsSuccess": "Synced {count} model(s) from {accounts} account(s); official prices filled for {priced}",
        "syncGroupModelsAlreadyUpToDate": "Group account models are already in pricing rules",
        "syncGroupModelsError": "Failed to sync group account models"
      }
    },
    "accounts": {
      "openai": {
        "codexCLIOnlyAllowClaudeCode": "Also allow Claude Code's Codex plugin",
        "codexCLIOnlyAllowClaudeCodeDesc": "Only takes effect when the switch above is on. Additionally allows requests from the Claude Code Codex plugin (exact match on originator=Claude Code) without weakening blocking of other non-official clients.",
        "codexImageGenerationBridge": "Codex image-generation bridge",
        "codexImageGenerationBridgeDesc": "Account policy takes precedence over channel and global settings. Only controls whether Codex requests through the /responses text endpoint receive the image_generation tool; standalone image-generation endpoints are unaffected.",
        "codexImageGenerationBridgeInherit": "Follow channel",
        "codexImageGenerationBridgeInheritDesc": "Do not write an account override; use the channel or global policy.",
        "codexImageGenerationBridgeEnabled": "Force on",
        "codexImageGenerationBridgeEnabledDesc": "Allow image tool injection for Codex /responses requests.",
        "codexImageGenerationBridgeDisabled": "Force off",
        "codexImageGenerationBridgeDisabledDesc": "Block image tool injection for Codex /responses requests.",
        "codexImageGenerationBridgeBadgeInherit": "Channel policy",
        "codexImageGenerationBridgeBadgeEnabled": "Account on",
        "codexImageGenerationBridgeBadgeDisabled": "Account off"
      }
    },
    "settings": {
      "gatewayForwarding": {
        "openaiAllowClaudeCodeCodexPlugin": "Allow using the Codex plugin in Claude Code",
        "openaiAllowClaudeCodeCodexPluginDesc": "Global switch; only affects OpenAI OAuth accounts that have 'Codex official clients only' enabled. When on, all such accounts additionally allow requests from the Claude Code Codex plugin (exact match on originator=Claude Code) without per-account config; upstream requests remain pass-through."
      },
      "site": {
        "externalRechargeEnabled": "Enable External Recharge Entry",
        "externalRechargeEnabledHint": "When enabled, the balance, the header \"Recharge\" button and the sidebar \"Recharge / Subscription\" item open the external URL below in a new tab. Falls back to the built-in recharge page when no URL is set.",
        "externalRechargeUrl": "External Recharge URL",
        "externalRechargeUrlHint": "External link opened in a new tab when recharging. Must start with http:// or https://. Leave empty to use the built-in recharge page.",
        "externalRechargeUrlPlaceholder": "https://your-recharge-site.com"
      }
    }
  }
} as const
