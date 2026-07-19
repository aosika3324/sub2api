package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户相关路由（需要认证）
func RegisterUserRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
) {
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	// 用户管理面变更类操作入审计（含 TOTP 启用/禁用、step-up 验证、密码修改等安全事件）
	authenticated.Use(gin.HandlerFunc(auditLog))
	{
		// 用户接口
		user := authenticated.Group("/user")
		{
			user.GET("/profile", h.User.GetProfile)
			user.PUT("/password", h.User.ChangePassword)
			user.PUT("", h.User.UpdateProfile)
			user.GET("/aff", h.User.GetAffiliate)
			user.POST("/aff/transfer", h.User.TransferAffiliateQuota)
			user.POST("/account-bindings/email/send-code", h.User.SendEmailBindingCode)
			user.POST("/account-bindings/email", h.User.BindEmailIdentity)
			user.DELETE("/account-bindings/:provider", h.User.UnbindIdentity)
			user.POST("/auth-identities/bind/start", h.User.StartIdentityBinding)
			user.GET("/api-keys/:id/usage/daily", h.Usage.GetMyAPIKeyDailyUsage)
			user.GET("/platform-quotas", h.User.GetMyPlatformQuotas)

			// 通知邮箱管理
			notifyEmail := user.Group("/notify-email")
			{
				notifyEmail.POST("/send-code", h.User.SendNotifyEmailCode)
				notifyEmail.POST("/verify", h.User.VerifyNotifyEmail)
				notifyEmail.PUT("/toggle", h.User.ToggleNotifyEmail)
				notifyEmail.DELETE("", h.User.RemoveNotifyEmail)
			}

			// TOTP 双因素认证
			totp := user.Group("/totp")
			{
				totp.GET("/status", h.Totp.GetStatus)
				totp.GET("/verification-method", h.Totp.GetVerificationMethod)
				totp.POST("/send-code", h.Totp.SendVerifyCode)
				totp.POST("/setup", h.Totp.InitiateSetup)
				totp.POST("/enable", h.Totp.Enable)
				totp.POST("/disable", h.Totp.Disable)
				// 敏感操作二次验证：授予当前会话一段时间的 step-up 权限
				totp.POST("/step-up", h.Totp.StepUp)
			}

			// 站内作图工作室（JWT，所有端点强制当前登录用户）
			imageStudio := user.Group("/image-studio")
			{
				imageStudio.POST("/generate", h.ImageStudio.Generate)
				imageStudio.GET("/conversations", h.ImageStudio.ListConversations)
				imageStudio.POST("/conversations", h.ImageStudio.CreateConversation)
				imageStudio.PATCH("/conversations/:id", h.ImageStudio.UpdateConversation)
				imageStudio.DELETE("/conversations/:id", h.ImageStudio.DeleteConversation)
				imageStudio.GET("/conversations/:id/generations", h.ImageStudio.ListConversationGenerations)
				imageStudio.GET("/generations", h.ImageStudio.ListGenerations)
				// Distinct top-level segment (not /generations/batch) so a static
				// segment never collides with the /generations/:id param route under
				// gin's httprouter.
				imageStudio.GET("/generations-batch", h.ImageStudio.BatchGetGenerations)
				imageStudio.GET("/generations/:id", h.ImageStudio.GetGeneration)
				imageStudio.DELETE("/generations/:id", h.ImageStudio.DeleteGeneration)
				imageStudio.DELETE("/history", h.ImageStudio.ClearHistory)
				imageStudio.GET("/assets/:genID/:idx", h.ImageStudio.GetAsset)
				imageStudio.GET("/input-assets/:genID/:idx", h.ImageStudio.GetInputAsset)
			}

			// 站内视频生成工作台（JWT，所有端点强制当前登录用户）
			// 与作图不同，Veo 是异步长任务：提交后通过读时轮询推进状态，
			// 完成时按视频秒数计费，视频按需经网关代理流式拉取（不落地存储）。
			videoStudio := user.Group("/video-studio")
			{
				videoStudio.POST("/generate", h.VideoStudio.Generate)
				videoStudio.GET("/generations", h.VideoStudio.ListGenerations)
				// Distinct top-level segment (not /generations/batch) so a static
				// segment never collides with the /generations/:id param route under
				// gin's httprouter.
				videoStudio.GET("/generations-batch", h.VideoStudio.BatchGetGenerations)
				videoStudio.GET("/generations/:id", h.VideoStudio.GetGeneration)
				videoStudio.GET("/generations/:id/video/:idx", h.VideoStudio.StreamVideo)
				videoStudio.DELETE("/generations/:id", h.VideoStudio.DeleteGeneration)
				videoStudio.DELETE("/history", h.VideoStudio.ClearHistory)
			}

			editableFiles := user.Group("/editable-files")
			{
				editableFiles.POST("/tasks", h.EditableFile.CreateTask)
				editableFiles.GET("/tasks", h.EditableFile.ListTasks)
				editableFiles.GET("/tasks/:id", h.EditableFile.GetTask)
				editableFiles.GET("/tasks/:id/artifacts", h.EditableFile.ListArtifacts)
			}
		}

		// API Key管理
		keys := authenticated.Group("/keys")
		{
			keys.GET("", h.APIKey.List)
			keys.GET("/:id", h.APIKey.GetByID)
			keys.POST("", h.APIKey.Create)
			keys.PUT("/:id", h.APIKey.Update)
			keys.DELETE("/:id", h.APIKey.Delete)
		}

		// 用户可用分组（非管理员接口）
		groups := authenticated.Group("/groups")
		{
			groups.GET("/available", h.APIKey.GetAvailableGroups)
			groups.GET("/rates", h.APIKey.GetUserGroupRates)
		}

		// 用户可用渠道（非管理员接口）
		channels := authenticated.Group("/channels")
		{
			channels.GET("/available", h.AvailableChannel.List)
		}

		// 用户计费透明度（非管理员接口）
		billing := authenticated.Group("/billing")
		{
			billing.GET("/rates", h.AvailableChannel.ListBillingRates)
		}

		// 使用记录
		usage := authenticated.Group("/usage")
		{
			usage.GET("", h.Usage.List)
			usage.GET("/errors", h.Usage.ListErrors)
			usage.GET("/errors/:id", h.Usage.GetErrorDetail)
			usage.GET("/:id", h.Usage.GetByID)
			usage.GET("/stats", h.Usage.Stats)
			// User dashboard endpoints
			usage.GET("/dashboard/stats", h.Usage.DashboardStats)
			usage.GET("/dashboard/trend", h.Usage.DashboardTrend)
			usage.GET("/dashboard/models", h.Usage.DashboardModels)
			usage.GET("/dashboard/snapshot-v2", h.Usage.DashboardSnapshotV2)
			usage.POST("/dashboard/api-keys-usage", h.Usage.DashboardAPIKeysUsage)
		}

		// 公告（用户可见）
		announcements := authenticated.Group("/announcements")
		{
			announcements.GET("", h.Announcement.List)
			announcements.POST("/:id/read", h.Announcement.MarkRead)
		}

		// 卡密兑换
		redeem := authenticated.Group("/redeem")
		{
			redeem.POST("", h.Redeem.Redeem)
			redeem.GET("/history", h.Redeem.GetHistory)
		}

		// 用户订阅
		subscriptions := authenticated.Group("/subscriptions")
		{
			subscriptions.GET("", h.Subscription.List)
			subscriptions.GET("/active", h.Subscription.GetActive)
			subscriptions.GET("/progress", h.Subscription.GetProgress)
			subscriptions.GET("/summary", h.Subscription.GetSummary)
		}

		// 渠道监控（用户只读）
		monitors := authenticated.Group("/channel-monitors")
		{
			monitors.GET("", h.ChannelMonitor.List)
			monitors.GET("/:id/status", h.ChannelMonitor.GetStatus)
		}
	}
}
