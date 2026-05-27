package middleware

import "strings"

var autoRouteEmbeddingTiers = []autoRouteEmbeddingTier{
	{
		Score: 15,
		Label: "light",
		Text:  "轻量任务：寒暄、简短常识、单句翻译、短文本润色、基础格式转换。",
		Samples: []string{
			"你好，今天适合喝咖啡吗？",
			"讲一个简短的冷笑话。",
			"法国的首都是哪里？",
			"爱因斯坦提出了什么理论？",
			"牛顿发现了什么？",
			"鲁迅是谁？",
			"老夫子是什么？",
			"水的沸点是多少？",
			"把 thank you 翻译成中文。",
			"把“今天天气很好”翻译成英文。",
			"把 hello world 转成大写。",
			"给“开心”找三个近义词。",
			"修正拼写：recieve 应该是什么？",
			"列出一周七天。",
			"25 乘以 4 等于多少？",
			"10% 的 250 是多少？",
			"7 是质数吗？",
			"What is 2 + 2?",
		},
	},
	{
		Score: 30,
		Label: "daily",
		Text:  "日常任务：普通问答、简短解释、轻量创作、简单总结、低风险代码片段。",
		Samples: []string{
			"什么是 GDP？请用普通人能懂的话解释。",
			"帮我润色这句话，让它更自然。",
			"把这段话总结成一句话。",
			"给我出一道二年级数学题。",
			"做一道简单的小学加减法题。",
			"用幽默风格出一道简单数学题。",
			"HTML 是编程语言吗？",
			"Python 列表推导式语法是什么？",
			"git status 是干什么的？",
			"npm install 的作用是什么？",
			"README.md 通常放什么内容？",
			"帮我写一封简短的客户通知邮件。",
			"把这段产品介绍改成更适合官网的文案。",
			"给这个接口加分页参数说明。",
			"写一个函数把秒数格式化为 HH:MM:SS。",
			"写一个正则提取字符串里的 URL。",
			"Translate 'good morning' to Spanish.",
			"Explain what a REST API is in simple terms.",
		},
	},
	{
		Score: 45,
		Label: "standard",
		Text:  "标准任务：常规解释、简单代码实现、单文件小改动、基础结构化输出。",
		Samples: []string{
			"解释一下什么是缓存穿透，并给一个简单例子。",
			"解释这段 JavaScript 函数在做什么。",
			"给这个 API 返回结构写一段 TypeScript 类型。",
			"写一个 Python 函数校验邮箱格式。",
			"给这个工具函数补一个单元测试。",
			"把这个 callback 改成 async/await。",
			"给文件读取逻辑加基础错误处理。",
			"写一个 SQL 查询，统计每个用户的订单数。",
			"修复这个循环里的 off-by-one 问题。",
			"写一个 debounce 函数。",
			"实现一个简单的内存缓存，支持 TTL。",
			"给 React 组件加 loading 状态。",
			"把这个函数拆成两个更清晰的小函数。",
			"给 Express 服务加 CORS 配置。",
			"实现二分查找并解释时间复杂度。",
			"解析一行 CSV，考虑带引号字段。",
			"Write a unit test for this utility function.",
			"Refactor this nested if-else into clearer branches.",
		},
	},
	{
		Score: 60,
		Label: "advanced",
		Text:  "进阶任务：普通排错、较多约束、局部方案设计、性能分析、跨文件小范围改造。",
		Samples: []string{
			"这个接口偶发 500，结合日志分析可能原因并给修复方案。",
			"分析这个 React 页面为什么流式返回不实时显示。",
			"给现有登录流程加邀请码字段，要求不影响老用户登录。",
			"把这个 Node 服务的同步文件处理改成流式处理。",
			"给 HTTP 调用加重试、超时和指数退避。",
			"帮我设计一个支付回调的幂等处理方案。",
			"分析这个 Docker 构建失败的原因并修改相关代码。",
			"把一个 REST endpoint 改成分页返回，并兼容旧参数。",
			"给请求体加 JSON Schema 校验，并返回统一错误格式。",
			"分析这个日志文件，统计错误类型并推断主要故障点。",
			"检查这个鉴权中间件是否存在绕过风险。",
			"设计一个通知模块的数据表和 API，支持已读状态。",
			"分析前端状态管理里导致重复请求的原因。",
			"设计一个订阅套餐和余额扣费的选择逻辑。",
			"给现有 CI 增加构建、测试和安全扫描步骤。",
			"Implement a rate limiter using a token bucket.",
			"Debug a memory leak in a small Node.js service.",
			"Optimize a slow query with basic index reasoning.",
		},
	},
	{
		Score: 75,
		Label: "expert",
		Text:  "高级任务：复杂代码排错、长上下文分析、并发/数据库优化、多步骤推理和多约束交付。",
		Samples: []string{
			"这段 Go 并发代码可能有数据竞争，帮我定位并修改。",
			"优化这个 SQL，表有 5000 万行，目前查询要 30 秒。",
			"实现一个线程安全的 LRU cache，支持容量限制和 TTL。",
			"排查为什么 Kubernetes pod 在高峰期被驱逐。",
			"把这个 800 行 service 拆分成更清晰的模块。",
			"我有一个 Go + React 的模型中转系统，流式响应偶发卡住，请结合 Nginx、SSE、前端 fetch 和计费日志定位瓶颈。",
			"设计一个自动模型路由算法，兼顾成本、质量、失败兜底、计费归因和 A/B 测试。",
			"分析一个跨模块权限 bug，要求指出根因、最小修复和回归测试。",
			"为现有支付系统补齐订阅和余额两套通道，要求幂等、可观测、异常可恢复。",
			"排查分布式缓存网络分区后读到旧数据的原因，并设计修复方案。",
			"审查一组 API 安全风险，包括鉴权、越权、注入和审计日志。",
			"把复杂前端聊天流式渲染重写成实时显示，保留非流式一次性显示。",
			"Investigate inconsistent cache reads after a network partition.",
			"Debug a production-only race condition in a Go service.",
			"Design a robust webhook pipeline with retries, idempotency, and observability.",
			"Plan a zero-downtime database migration for a high-traffic table.",
			"Analyze a large TypeScript refactor and propose safe incremental changes.",
			"Diagnose latency across proxy, backend, upstream model, and frontend streaming.",
		},
	},
	{
		Score: 90,
		Label: "flagship",
		Text:  "旗舰任务：系统架构、复杂证明、长上下文代码库改造、多工具规划、高风险技术决策。",
		Samples: []string{
			"设计一个支持亿级请求的模型中转平台，包括路由、计费、限流、审计和故障降级。",
			"为金融交易系统设计事件溯源架构，要求高吞吐、强审计和灾备。",
			"设计一个多云零信任安全架构，兼容本地遗留系统。",
			"把单体 PHP 应用迁移到云原生架构，给出阶段计划、风险和回滚策略。",
			"设计实时多人游戏后端，包括匹配、排行榜、反作弊和状态同步。",
			"为 100 million daily requests 的短链系统设计架构和数据分层。",
			"分析 Paxos、Raft、PBFT 在不同分布式系统里的取舍。",
			"证明 Dijkstra 算法正确性，并比较不同优先队列实现的复杂度。",
			"从第一性原理推导反向传播，并解释自动微分如何实现。",
			"证明停机问题不可判定，并说明对软件验证的影响。",
			"审计完整 OAuth2 + PKCE 移动端登录方案，做威胁建模并给修复建议。",
			"重构认证系统，从 JWT 改成 session，覆盖中间件、路由、测试和迁移。",
			"分析一个机器学习流水线的数据泄漏风险，并重新设计评估流程。",
			"设计跨地区金融系统灾备方案，RPO 15 分钟，RTO 1 小时。",
			"评估大型 React 应用离线优先场景下的状态管理方案取舍。",
			"把 2000 行 God class 按 DDD 拆成 bounded context、aggregate 和 event handler。",
			"为医疗 AI 诊断公司设计数据治理和隐私合规技术方案。",
			"为 500 个微服务选择 Kubernetes、Nomad 或 ECS，并给迁移策略。",
			"Design a scalable notification system across push, email, SMS, and in-app channels.",
			"Perform a security audit covering OWASP Top 10 risks with concrete mitigations.",
		},
	},
}

func autoRouteEmbeddingTierText(tier autoRouteEmbeddingTier) string {
	prototypes := autoRouteEmbeddingTierPrototypes(tier)
	if len(prototypes) == 0 {
		return tier.Text
	}
	if tier.Text == "" {
		return strings.Join(prototypes, "\n")
	}
	return tier.Text + "\n" + strings.Join(prototypes, "\n")
}

func autoRouteEmbeddingTierPrototypes(tier autoRouteEmbeddingTier) []string {
	if len(tier.Samples) > 0 {
		return tier.Samples
	}
	if strings.TrimSpace(tier.Text) == "" {
		return nil
	}
	return []string{tier.Text}
}
