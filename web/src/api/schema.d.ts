export interface paths {
    "/factory/sessions": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listFactorySessions"];
        put?: never;
        post: operations["createFactorySession"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/factory/sessions/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["getFactorySession"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/factory/sessions/{id}/messages": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["generateFactoryProposal"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/factory/sessions/{id}/apply": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["applyFactoryProposal"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/fork": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["forkRun"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/fork/preview": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["previewFork"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/inputs": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["runInputs"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/workflows/preview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["previewInputs"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/github/hooks": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listGithubHooks"];
        put?: never;
        post: operations["createGithubHook"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/github/hooks/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        delete: operations["disableGithubHook"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/github/hooks/{id}/events": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["githubEvents"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/hooks/github/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description 独立 HMAC-SHA256 认证，仅接收固定仓库与标签的 issues labeled 事件；原始体最多 256 KiB，Issue 正文最多 12000 字节。 */
        post: operations["receiveGithubIssue"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/usage": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["runUsage"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/reports/{report}/deliveries": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                report: string;
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description 从已成功的设计报告固定选中任务与实施/只读审查版本；不会自动批准或执行。 */
        post: operations["createDelivery"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/deliveries": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listDeliveries"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/deliveries/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["getDelivery"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/deliveries/{id}/run": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description 每份草案只产生一个幂等运行，实施节点仍需飞书人工批准。 */
        post: operations["runDelivery"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/deliveries/{id}/artifacts.zip": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["exportDelivery"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/reports": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listReports"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/reports/{report}/claims/{claim}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                report: string;
                claim: string;
            };
            cookie?: never;
        };
        get?: never;
        /** @description 运行所有者或管理者按报告摘要与修订号登记人工核验；核验记录随报告正文清理。 */
        put: operations["reviewClaim"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/approvals": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listApprovals"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/approvals/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get?: never;
        /** @description 飞书会话中的运行所有者或管理者批准/驳回；开始执行前可以撤回批准。API Token 不可代替人工审批。 */
        put: operations["decideApproval"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/feedback": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["getFeedback"];
        put: operations["saveFeedback"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/quality": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["projectQuality"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/evaluations/cases": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listEvaluationCases"];
        put?: never;
        post: operations["createEvaluationCase"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/evaluations/cases/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["getEvaluationCase"];
        put?: never;
        post?: never;
        delete: operations["retireEvaluationCase"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/evaluations/cases/{id}/runs": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listEvaluationRuns"];
        put?: never;
        post: operations["runEvaluation"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runnables": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description 读取工作空间未归档的角色和工作流，每个对象只返回最新已发布版本。 */
        get: operations["runnableChoices"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/models/codex": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description 已登录用户读取 Worker 当前 Codex 程序的内置可显示模型目录；每次重新读取，不使用模型缓存、不调用收费模型。账号可用性由实际提供方决定。 */
        get: operations["codexModelChoices"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/connections/choices": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description 已登录用户选择共享模型版本；不返回端点或凭据引用。 */
        get: operations["connectionChoices"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listProjects"];
        put?: never;
        post: operations["registerProject"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/healthz": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["getHealth"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/readyz": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["getReadiness"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/openapi.yaml": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["getContract"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/workflows/validate": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["validateWorkflow"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listRuns"];
        put?: never;
        post: operations["submitRun"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/requests/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["getSubmission"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/workflows/preflight": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["preflightWorkflow"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/cancel": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["cancelRun"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["getRun"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/attempts": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listAttempts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/nodes/{node}/attempts/{attempt}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                node: string;
                attempt: number;
            };
            cookie?: never;
        };
        get: operations["getAttempt"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/artifacts": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listArtifacts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/artifacts/{artifact}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                artifact: string;
            };
            cookie?: never;
        };
        get: operations["downloadArtifact"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/artifacts/{artifact}/diff": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                artifact: string;
            };
            cookie?: never;
        };
        get: operations["getArtifactDiff"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/artifacts.zip": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["exportArtifacts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/crons": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listCrons"];
        put?: never;
        post: operations["createCron"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/crons/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["getCron"];
        put: operations["editCron"];
        post?: never;
        delete: operations["deleteCron"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/crons/{id}/fires": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listScheduleFires"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/catalog": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listCatalog"];
        put?: never;
        post: operations["createCatalog"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/catalog/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["getCatalog"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/catalog/{id}/draft": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put: operations["saveCatalog"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/catalog/{id}/draft/preflight": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description 检查已保存的 Agent 修订和 Worker，不调用收费模型。 */
        post: operations["preflightAgentDraft"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/catalog/{id}/trials": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        get: operations["listAgentTrials"];
        put?: never;
        /** @description 原子保存已存在草稿修订的执行快照和试运行来源，复用提交恢复机制；不创建发布版本。Idempotency-Key 使用 Unix 毫秒时间戳.UUID；超过 89 天或超前服务器 5 分钟的键被拒绝，清理记录后也不重发旧请求。 */
        post: operations["runAgentDraft"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/catalog/{id}/publish": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["publishCatalog"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/catalog/{id}/archive": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["archiveCatalog"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/catalog/{id}/versions": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description 每页最多 100 条；offset 翻页，按发布序号倒序。 */
        get: operations["catalogVersions"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/versions/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["catalogVersion"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/versions/{id}/run": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["runVersion"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/rerun": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description 从原定义及原 Git 基准整次执行，新建运行；不接受请求体或查询参数。 */
        post: operations["rerun"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/crons/preview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["cronPreview"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/catalog/import": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["importCatalog"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/catalog/{id}/export": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["exportCatalog"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/versions/{id}/export": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["exportCatalogVersion"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/me": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["getIdentity"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/logout": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["logout"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/memberships": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listMemberships"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/tokens": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listAPITokens"];
        put?: never;
        post: operations["issueAPIToken"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/tokens/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        delete: operations["revokeAPIToken"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/members": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listMembers"];
        put: operations["grantMember"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/members/{user}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        delete: operations["removeMember"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/users/{id}/disable": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["disableUser"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/users/{id}/enable": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["enableUser"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/login": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["feishuLogin"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/callback": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["feishuCallback"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/audit": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listAudit"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/versions/{id}/preflight": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description 使用指定项目的 Worker 检查共享 Agent、工作流或模型连接；不调用收费模型。 */
        post: operations["preflightVersion"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/events": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description 按根运行或其直接子节点分页读取脱敏调度事件，禁止跨运行节点查询。 */
        get: operations["runEvents"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/retention": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description 管理员预览当前 Worker 最旧的最多100条候选；活动运行受保护。 */
        get: operations["previewRetention"];
        put?: never;
        /** @description 管理员执行一批清理；每项独立检查终态、租约和托管目录，中断可重试。 */
        post: operations["executeRetention"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/retention/history": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description 管理员分页查询独立保留180天的清理记录。 */
        get: operations["listRetentionHistory"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/activity": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        /** @description 每个活动节点最近开始且尚未结束的操作。仅包含未结束的实际尝试，按节点返回至多一条；不受 trace 分页影响，并行操作的全部记录可按节点查询 trace。复用运行读取权限及正文保留期。 */
        get: operations["runActivity"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/trace": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        /** @description 分页读取实际行为，每条事件关联运行、节点、尝试及父 span；正文在写入前脱敏和截断，不记录内部思维链。 */
        get: operations["runTrace"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/trace.ndjson": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        /** @description 导出当前筛选下的全部追踪事件，按序号倒序；开始后新增事件不纳入。文件末尾 export_error 表示导出中断。 */
        get: operations["exportTrace"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/debug": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["runDebug"];
        put?: never;
        /** @description 需要 manage_run 权限；暂停阻止后续节点开始，已执行的节点继续。单步放行一个节点尝试（失败重试也重新等待许可）。状态版本不符或仍有活动节点时返回 409。 */
        post: operations["controlDebug"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        DebugState: {
            paused: boolean;
            step_requested: boolean;
            step_node: string;
            revision: number;
            active: number;
        };
        TracePage: {
            items: components["schemas"]["TraceEvent"][];
            /** Format: int64 */
            next_before?: number;
        };
        TraceEvent: {
            /** Format: int64 */
            id: number;
            /** Format: uuid */
            run_id: string;
            node_id: string;
            /** Format: uuid */
            node_run_id: string;
            attempt: number;
            span_id: string;
            parent_span_id: string;
            /** @enum {string} */
            kind: "node" | "model" | "tool";
            /** @enum {string} */
            phase: "start" | "end";
            name: string;
            /** Format: date-time */
            time: string;
            /** Format: int64 */
            duration_ms: number;
            /** @enum {string} */
            status: "running" | "succeeded" | "failed" | "cancelled";
            input: string;
            output: string;
            error: string;
            truncated: boolean;
        };
        FactoryInput: {
            /** Format: uuid */
            id: string;
            revision: number;
            message: string;
            /** @enum {string} */
            mode: "build" | "modify" | "diagnose" | "optimize";
            /** Format: uuid */
            connection_version: string;
            /** Format: uuid */
            target_id?: string;
            target_revision?: number;
            /** Format: uuid */
            source_run_id?: string;
        };
        FactoryAgent: {
            key: string;
            name: string;
            /** Format: uuid */
            object_id?: string;
            revision?: number;
            /** Format: uuid */
            version_id?: string;
            node?: components["schemas"]["Node"];
        };
        FactoryStep: {
            id: string;
            agent: string;
            depends_on?: string[];
            bindings?: {
                [key: string]: components["schemas"]["FieldBinding"];
            };
            when?: components["schemas"]["Condition"];
            approval?: components["schemas"]["ApprovalPolicy"];
        };
        FactoryProposal: {
            name: string;
            agents: components["schemas"]["FactoryAgent"][];
            steps?: components["schemas"]["FactoryStep"][];
            /** Format: uuid */
            object_id?: string;
            revision?: number;
            parameters?: components["schemas"]["Shape"];
            budget?: components["schemas"]["Budget"];
        };
        FactoryReply: {
            message: string;
            proposal?: components["schemas"]["FactoryProposal"];
        };
        FactoryResult: {
            agents: components["schemas"]["CatalogVersion"][];
            workflow?: components["schemas"]["CatalogVersion"];
        };
        FactoryTurn: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            request_id?: string;
            input: components["schemas"]["FactoryInput"];
            skill_digest: string;
            /** @enum {string} */
            status: "pending" | "ready" | "failed" | "applied" | "expired";
            reply?: components["schemas"]["FactoryReply"];
            error?: string;
            result?: components["schemas"]["FactoryResult"];
            /** Format: date-time */
            created_at: string;
        };
        FactorySession: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            project_id: string;
            created_by?: string;
            title: string;
            revision: number;
            /** Format: date-time */
            updated_at: string;
            turns: components["schemas"]["FactoryTurn"][];
        };
        FactorySessionPage: {
            items: components["schemas"]["FactorySession"][];
            next_offset?: number;
        };
        Condition: {
            field: components["schemas"]["FieldBinding"];
            equals: boolean | string;
        };
        InputPreview: {
            node_id: string;
            /** @enum {string} */
            state: "ready" | "pending" | "skipped" | "invalid";
            parameters: {
                [key: string]: unknown;
            };
            error: string;
        };
        ForkInput: {
            checkpoint_node: string;
            workflow: components["schemas"]["Workflow"];
        };
        ForkPreview: {
            reused_nodes: string[];
            execute_nodes: string[];
            input_changed: boolean;
        };
        Fork: {
            request_hash: string;
            checkpoint_node: string;
            commit: string;
            origin_commit: string;
            checkpoints?: {
                [key: string]: string;
            };
            reused?: {
                [key: string]: {
                    text: string;
                    run_id: string;
                    data?: {
                        [key: string]: unknown;
                    };
                };
            };
        };
        GithubHookConfig: {
            /** Format: uuid */
            project_id: string;
            /** Format: uuid */
            version_id: string;
            /** @example owner/repo */
            repository: string;
            label: string;
            secret_ref: string;
            enabled: boolean;
        };
        GithubHook: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            actor_id: string;
            /** Format: uuid */
            project_id: string;
            /** Format: uuid */
            version_id: string;
            repository: string;
            label: string;
            secret_ref: string;
            enabled: boolean;
            /** Format: date-time */
            created_at: string;
        };
        GithubHookPage: {
            items: components["schemas"]["GithubHook"][];
            next_offset?: number;
        };
        GithubEventPage: {
            next_offset?: number;
            items: {
                /** Format: uuid */
                delivery_id: string;
                /** Format: uuid */
                request_id: string;
                run_id: string;
                /** Format: date-time */
                created_at: string;
            }[];
        };
        Budget: {
            model_calls: number;
            search_calls: number;
            codex_sessions: number;
        };
        Price: {
            /** @enum {string} */
            currency: "USD" | "CNY";
            input_per_million_micros: number;
            output_per_million_micros: number;
        };
        UsageCall: {
            /** Format: uuid */
            id: string;
            node_id: string;
            /** Format: uuid */
            node_run_id: string;
            attempt: number;
            /** @enum {string} */
            kind: "model" | "search" | "codex";
            model: string;
            /** @enum {string} */
            status: "unknown" | "completed" | "reported";
            price: components["schemas"]["Price"] | null;
            input_tokens: number | null;
            output_tokens: number | null;
            cached_tokens: number | null;
            estimate_micros: number | null;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            finished_at: string | null;
        };
        UsageReport: {
            /** @description 未主动设置调用预算时为 null，不施加平台默认上限。 */
            budget: components["schemas"]["Budget"] | null;
            used: components["schemas"]["Budget"];
            calls: components["schemas"]["UsageCall"][];
            next_offset?: number;
        };
        DeliveryInput: {
            subject_hash: string;
            task_ids: string[];
            /** Format: uuid */
            implementation_version: string;
            /** Format: uuid */
            review_version: string;
        };
        Delivery: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            project_id: string;
            /** Format: uuid */
            actor_id: string;
            /** Format: uuid */
            source_run_id: string;
            /** Format: uuid */
            report_id: string;
            report_hash: string;
            subject_hash: string;
            definition: components["schemas"]["Workflow"] | null;
            request_id: string;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            expires_at: string;
        };
        DeliveryPage: {
            items: components["schemas"]["Delivery"][];
            next_offset?: number;
        };
        ReportEvidence: {
            source_id: string;
            quote: string;
            /** @description 模型标注的资料日期 YYYY-MM-DD 或 unknown，尚需人工确认。 */
            source_date: string;
        };
        ReportClaim: {
            id: string;
            statement: string;
            /** @enum {string} */
            kind: "fact" | "inference" | "unknown";
            scope: string;
            uncertainty: string;
            conflicts: string[] | null;
            evidence: components["schemas"]["ReportEvidence"][] | null;
        };
        ReportSource: {
            id: string;
            title: string;
            read: boolean;
            url?: string;
            /** Format: date-time */
            retrieved_at?: string;
            sha256?: string;
            artifact_id?: string;
        };
        DesignTask: {
            id: string;
            title: string;
            scope: string;
            depends_on: string[] | null;
            deliverable: string;
            acceptance: string;
        };
        StructuredReport: {
            mode: string;
            tasks: components["schemas"]["DesignTask"][] | null;
            claims: components["schemas"]["ReportClaim"][] | null;
            sources: components["schemas"]["ReportSource"][] | null;
        };
        ClaimReviewInput: {
            revision: number;
            subject_hash: string;
            /** @enum {string} */
            verdict: "supported" | "partial" | "unsupported" | "outdated";
            reason: string;
        };
        ClaimReview: components["schemas"]["ClaimReviewInput"] & {
            /** @enum {string} */
            source: "unknown" | "feishu_session";
            claim_id: string;
            /** Format: uuid */
            actor_id: string;
            /** Format: date-time */
            updated_at: string;
        };
        ReportDetail: {
            /** Format: uuid */
            id: string;
            title: string;
            subject_hash: string;
            data: components["schemas"]["StructuredReport"] | null;
            reviews: components["schemas"]["ClaimReview"][];
        };
        ApprovalPolicy: {
            /** @description 节点之前独立的人工审批等待期限。 */
            timeout_seconds: number;
        };
        ApprovalDecision: {
            subject_hash: string;
            /** @enum {string} */
            decision: "approved" | "rejected" | "revoked";
            reason: string;
        };
        ApprovalGate: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            project_id: string;
            /** Format: uuid */
            run_id: string;
            node_id: string;
            owner_id: string;
            subject_hash: string;
            subject: {
                [key: string]: unknown;
            } | null;
            head_commit: string;
            /** @enum {string} */
            status: "waiting" | "approved" | "rejected" | "revoked" | "consumed" | "expired";
            actor_id: string;
            reason: string;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            expires_at: string;
            /** Format: date-time */
            decided_at: string | null;
            /** Format: date-time */
            consumed_at: string | null;
        };
        FeedbackInput: {
            revision: number;
            subject_hash: string;
            /** @enum {string} */
            decision: "accepted" | "needs_changes" | "rejected";
            reason: string;
        };
        Feedback: components["schemas"]["FeedbackInput"] & {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            run_id: string;
            /** Format: uuid */
            task_id: string;
            /** Format: uuid */
            actor_id: string;
            /** @enum {string} */
            first_decision: "accepted" | "needs_changes" | "rejected";
            /** @enum {string} */
            source: "unknown" | "feishu_session";
            payload_expired: boolean;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            updated_at: string;
        };
        FeedbackState: {
            subject_hash: string;
            can_accept: boolean;
            feedback: components["schemas"]["Feedback"] | null;
        };
        Quality: {
            tasks: number;
            runs: number;
            first_reviewed: number;
            first_accepted: number;
            reviewed_tasks: number;
            accepted_tasks: number;
            needs_changes: number;
            rejected_tasks: number;
            unreviewed_tasks: number;
        };
        EvaluationCaseInput: {
            /** Format: uuid */
            project_id: string;
            name: string;
            input: string;
            expected: string;
            criteria: string[];
            /**
             * @description 明确同意将输入、预期和标准保留 30 天；到期清除正文，元数据最多 90 天。
             * @enum {boolean}
             */
            retain_input: true;
        };
        EvaluationCase: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            project_id: string;
            /** Format: uuid */
            actor_id: string;
            name: string;
            input: string;
            expected: string;
            criteria: string[];
            payload_expired: boolean;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            expires_at: string;
        };
        EvaluationRun: {
            /** Format: uuid */
            case_id: string;
            /** Format: uuid */
            version_id: string;
            /** Format: uuid */
            request_id: string;
            run_id: string;
            /** @enum {string} */
            decision: "" | "accepted" | "needs_changes" | "rejected";
            /** Format: date-time */
            created_at: string;
        };
        EvaluationCasePage: {
            items: components["schemas"]["EvaluationCase"][];
            next_offset?: number;
        };
        EvaluationRunPage: {
            items: components["schemas"]["EvaluationRun"][];
            next_offset?: number;
        };
        RunnableChoicePage: {
            next_offset?: number;
            items: {
                name: string;
                version: components["schemas"]["CatalogVersion"];
            }[];
        };
        ModelChoicePage: {
            items: {
                id: string;
                name: string;
            }[];
        };
        ConnectionChoicePage: {
            next_offset?: number;
            items: {
                /** Format: uuid */
                id: string;
                name: string;
                number: number;
                model: string;
            }[];
        };
        AgentTrialInput: {
            /** Format: uuid */
            project_id: string;
            revision: number;
            input: string;
        };
        AgentTrial: {
            /** Format: uuid */
            request_id: string;
            /** Format: uuid */
            run_id?: string;
            revision: number;
            /** Format: uuid */
            created_by?: string;
            /** Format: date-time */
            created_at: string;
            /** @enum {string} */
            status: "pending" | "accepted";
        };
        AgentTrialPage: {
            items: components["schemas"]["AgentTrial"][];
            next_offset?: number;
        };
        Submission: {
            /** Format: uuid */
            task_id: string;
            /** Format: uuid */
            request_id: string;
            /** Format: uuid */
            project_id: string;
            /** @enum {string} */
            operation: "submit" | "cron" | "rerun";
            /** @enum {string} */
            status: "pending" | "accepted";
            /** Format: uuid */
            run_id?: string;
            /** Format: date-time */
            created_at: string;
            /** Format: uuid */
            created_by?: string;
        };
        Schedule: {
            /** Format: uuid */
            id: string;
            name: string;
            expression: string;
            workflow: components["schemas"]["Workflow"];
            revision: number;
            paused: boolean;
            deleted: boolean;
            ready: boolean;
            /** Format: date-time */
            created_at: string;
            /** Format: uuid */
            created_by?: string;
        };
        ScheduleFire: {
            /** Format: uuid */
            schedule_id: string;
            /** Format: date-time */
            slot: string;
            /** @enum {string} */
            status: "pending" | "accepted";
            reason: string;
            /** Format: uuid */
            request_id?: string;
        };
        ProjectRegistration: {
            name: string;
            /** @description 绑定 Worker 上的本地 Git 项目绝对路径 */
            source: string;
            /** @description 本地分支，不自动 fetch */
            branch: string;
            worker_id: string;
            /** @description 管理员登记的验证命令参数数组，首项为Worker上的绝对程序路径；每个Codex节点成功后、检查点发布前执行，最长30秒。禁止凭据参数。不配置时为空数组。 */
            verification?: string[];
        };
        Project: {
            /** Format: uuid */
            id: string;
            name: string;
            source: string;
            branch: string;
            worker_id: string;
            /** @description 管理员登记的验证命令参数数组，首项为Worker上的绝对程序路径；每个Codex节点成功后、检查点发布前执行，最长30秒。禁止凭据参数。不配置时为空数组。 */
            verification?: string[];
        };
        Preflight: {
            ready: boolean;
            checks: {
                component: string;
                /** @enum {string} */
                status: "ready" | "configured" | "unavailable";
            }[];
        };
        ArtifactFile: {
            /** @description Git 文件为 32 位十六进制摘要；产品经理报告为 UUID。 */
            id: string;
            /** @description 仓库相对路径，不是本机绝对路径 */
            path: string;
            /** @enum {string} */
            change: "A" | "M" | "D" | "T";
            /** Format: int64 */
            size: number;
            sha256?: string;
            binary: boolean;
            /** @enum {string} */
            blocked?: "sensitive" | "symlink" | "unsafe_path" | "too_large";
        };
        ArtifactManifest: {
            /** Format: uuid */
            run_id: string;
            base_commit: string;
            head_commit: string;
            files: components["schemas"]["ArtifactFile"][];
            /** Format: int64 */
            total_bytes: number;
        };
        AttemptPage: {
            items: components["schemas"]["Attempt"][];
            next_offset?: number;
        };
        Attempt: {
            /** Format: uuid */
            run_id: string;
            /** Format: uuid */
            node_run_id: string;
            node_id: string;
            attempt: number;
            /** Format: date-time */
            started_at: string;
            /** Format: date-time */
            finished_at?: string;
            /** @enum {string} */
            status: "running" | "succeeded" | "failed" | "cancelled";
            message: string;
            truncated: boolean;
            evidence_truncated: boolean;
            evidence: {
                id: string;
                command: string;
                exit_code: number;
                output: string;
                truncated: boolean;
            }[];
            category: string;
        };
        RunSummary: {
            /** @description 从任务输入提取的简短标题；过期后不再返回。 */
            title?: string;
            /** Format: uuid */
            id: string;
            /** @enum {string} */
            status: "queued" | "running" | "cancelling" | "succeeded" | "failed" | "cancelled" | "skipped" | "unknown";
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            started_at?: string;
            /** Format: date-time */
            finished_at?: string;
            /** Format: uuid */
            created_by?: string;
            /** Format: uuid */
            project_id?: string;
        };
        RunError: {
            category: string;
            /** @description 已脱敏的诊断，最多64KiB */
            message: string;
            truncated: boolean;
        };
        NodeRun: components["schemas"]["RunSummary"] & {
            /** Format: uuid */
            reused_from?: string;
            node_id: string;
            /** Format: uuid */
            task_id: string;
            executor: string;
            /** @description 真实执行尝试从1开始，跳过或复用检查点为0，任务ID为派生显示标识 */
            attempt: number;
            output?: string;
            error?: components["schemas"]["RunError"];
        };
        RunEvent: {
            id: number;
            /** Format: uuid */
            task_id: string;
            attempt: number;
            type: string;
            /** Format: date-time */
            time: string;
            message: string;
            truncated: boolean;
        };
        RunDetails: components["schemas"]["RunSummary"] & {
            fork?: components["schemas"]["Fork"];
            nodes: components["schemas"]["NodeRun"][];
            events: components["schemas"]["RunEvent"][];
            attempts: components["schemas"]["Attempt"][];
            attempts_next_offset?: number;
            cancel_requested: boolean;
            /** @description 根/子任务终态且没有未确认停止的执行尝试 */
            stop_confirmed: boolean;
            error?: components["schemas"]["RunError"];
            definition?: components["schemas"]["Workflow"];
            /** Format: uuid */
            source_run_id?: string;
            /** @description 日志、产物与重跑基准已到期。 */
            logs_expired: boolean;
        };
        RunList: {
            items: components["schemas"]["RunSummary"][];
            next_offset?: number;
            /** Format: date-time */
            since: string;
            /** Format: date-time */
            until: string;
        };
        Health: {
            /** @enum {string} */
            status: "ok";
        };
        Readiness: {
            /** @enum {string} */
            status: "ready" | "not_ready";
            /** @enum {string} */
            scope: "api_dependencies";
            checks: {
                /** @enum {string} */
                database: "ok" | "unavailable";
                /** @enum {string} */
                hatchet_registration: "ok" | "unavailable";
            };
        };
        Error: {
            error: string;
        };
        Validation: {
            /** @enum {boolean} */
            valid: true;
            layers: string[][];
        };
        Workflow: {
            budget?: components["schemas"]["Budget"];
            /**
             * Format: uuid
             * @description Codex 编排必填；必须为当前命名空间登记的项目
             */
            project_id?: string;
            input?: string;
            nodes: components["schemas"]["Node"][];
            /** Format: uuid */
            version_id?: string;
            input_schema?: components["schemas"]["Shape"];
        };
        Node: {
            handoff?: components["schemas"]["HandoffContract"];
            when?: components["schemas"]["Condition"];
            approval?: components["schemas"]["ApprovalPolicy"];
            id: string;
            /** @enum {string} */
            executor: "eino" | "codex";
            /** @description 非空，最多 16 KiB */
            instruction: string;
            /** @description 最多 1 KiB */
            description?: string;
            /** @description 无模型连接的 Eino 必填；有连接时须与连接模型一致，Codex 账号连接可留空使用默认模型。 */
            model?: string;
            depends_on?: string[];
            /** Format: uuid */
            agent_version?: string;
            /** Format: uuid */
            connection_version?: string;
            output_schema?: components["schemas"]["Shape"];
            bindings?: {
                [key: string]: components["schemas"]["FieldBinding"];
            };
            policy?: components["schemas"]["Policy"];
            /** @enum {string} */
            role?: "product_manager";
            /** @enum {string} */
            mode?: "research" | "competition" | "planning" | "design";
            /** @description 仅 Codex：设置原生 read-only 沙箱；写入变更不能发布检查点。 */
            read_only?: boolean;
        } & (({
            /** @enum {string} */
            executor?: "eino";
        } | unknown | {
            model: string;
        }) | {
            /** @enum {string} */
            executor?: "codex";
        });
        /** @description 交接契约。草稿允许缺项或留空；发布、试运行及新任务提交时五项均须为非空白文本，每项最多 4096 UTF-8 字节。状态、权限和预算由既有运行配置控制，契约描述不授予额外权限。 */
        HandoffContract: {
            /** @description 任务目标与职责边界 */
            objective?: string;
            /** @description 必需输入与上下文 */
            inputs?: string;
            /** @description 交付物及其格式 */
            deliverables?: string;
            /** @description 可核验的验收标准 */
            acceptance?: string;
            /** @description 缺失资料与失败时如何处理 */
            failure_handling?: string;
        };
        /** @description 无引用与脚本的 JSON Schema 子集；对象拒绝未知字段，最多 6 层、每层 64 个字段、整体 16 KiB。 */
        Shape: {
            /** @enum {string} */
            type: "object" | "array" | "string" | "number" | "integer" | "boolean" | "null";
            properties?: {
                [key: string]: components["schemas"]["Shape"];
            };
            required?: string[];
            items?: components["schemas"]["Shape"];
        };
        /** @description 只读取 $input 或声明的直接上游；pointer 为 JSON Pointer，无隐式类型转换。 */
        FieldBinding: {
            from: string;
            pointer: string;
            type: string;
        };
        /** @description openai 使用服务端凭据引用；codex 使用 Worker 专用登录且模型可留空，禁止配置 API 端点、凭据引用与单价。执行时读取原始来源，不返回密钥。 */
        Connection: {
            /** @enum {string} */
            provider?: "openai" | "codex";
            price?: components["schemas"]["Price"];
            /** Format: uri */
            base_url?: string;
            model?: string;
            secret_ref?: string;
        } & ({
            /** @enum {unknown} */
            provider?: "openai";
            model: unknown;
        } | {
            /** @enum {unknown} */
            provider: "codex";
        });
        FlowBinding: {
            /** @description 本步骤任务，追加到角色指令；最多 8192 字节，合并后指令最多 16384 字节 */
            task?: string;
            /** @description 仅可收紧为只读，不能解除助手已有的只读权限 */
            read_only?: boolean;
            output_schema?: components["schemas"]["Shape"];
            when?: components["schemas"]["Condition"];
            approval?: components["schemas"]["ApprovalPolicy"];
            id: string;
            /** Format: uuid */
            agent_version: string;
            depends_on?: string[];
            bindings?: {
                [key: string]: components["schemas"]["FieldBinding"];
            };
        };
        /** @description 仅用于画布展示，与草稿及发布版本一起保存，执行编译忽略。 */
        FlowEditor: {
            nodes?: {
                [key: string]: {
                    /** @description 节点显示名称，最多 128 字节 */
                    name?: string;
                    x: number;
                    y: number;
                };
            };
            viewport?: {
                x: number;
                y: number;
                zoom: number;
            };
        };
        Flow: {
            editor?: components["schemas"]["FlowEditor"];
            budget?: components["schemas"]["Budget"];
            parameters?: components["schemas"]["Shape"];
            nodes: components["schemas"]["FlowBinding"][];
        };
        CatalogSpec: {
            agent?: components["schemas"]["Node"];
            flow?: components["schemas"]["Flow"];
            connection?: components["schemas"]["Connection"];
        };
        CatalogCreate: {
            /** @enum {string} */
            kind: "agent" | "workflow" | "connection";
            name: string;
            spec: components["schemas"]["CatalogSpec"];
        };
        CatalogObject: {
            /** @enum {string} */
            kind: "agent" | "workflow" | "connection";
            name: string;
            spec: components["schemas"]["CatalogSpec"];
            /** Format: uuid */
            id: string;
            revision: number;
            archived: boolean;
            /** Format: date-time */
            created_at: string;
        };
        CatalogVersion: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            object_id: string;
            kind: string;
            number: number;
            source_revision: number;
            spec: components["schemas"]["CatalogSpec"];
            /** Format: date-time */
            created_at: string;
            archived: boolean;
        };
        /** @description 只控制失败重试，缺省 2 次；不设置工具次数或执行时限。 */
        Policy: {
            retries: number;
        };
        CatalogDocument: {
            /** @enum {string} */
            format: "pangu.catalog.v1";
            /** @enum {string} */
            kind: "agent" | "workflow" | "connection";
            name: string;
            spec: components["schemas"]["CatalogSpec"];
        };
        /** @description 产品经理 input 使用此结构的 JSON 字符串；只有任务目标必填，省略背景表示未提供，不推测用户资料。各列表最多 10 项。竞品模式要求 competitors/dimensions，调研要求至少一个授权资料来源。 */
        ResearchBrief: {
            question: string;
            audience?: string;
            market?: string;
            time_scope?: string;
            context?: string;
            constraints?: string;
            public_queries?: string[];
            urls?: string[];
            documents?: string[];
            competitors?: string[];
            dimensions?: string[];
        };
        Actor: {
            /** Format: uuid */
            id: string;
            name: string;
            admin: boolean;
            csrf?: string;
        };
        Membership: {
            /** Format: uuid */
            project_id: string;
            /** @enum {string} */
            role: "manager" | "operator" | "viewer";
        };
        Member: {
            /** Format: uuid */
            id: string;
            open_id: string;
            name: string;
            /** @enum {string} */
            role: "manager" | "operator" | "viewer";
            enabled: boolean;
            disabled: boolean;
        };
        APIToken: {
            /** Format: uuid */
            id: string;
            name: string;
            /** Format: uuid */
            project_id: string;
            scopes: ("read" | "run" | "configure" | "schedule" | "manage_run" | "manage_schedule" | "manage_factory" | "manage_connections")[];
            /** Format: date-time */
            expires_at: string;
            /** Format: date-time */
            revoked_at?: string;
        };
        TokenIssue: {
            /** Format: uuid */
            project_id: string;
            name: string;
            scopes: ("read" | "run" | "configure" | "schedule" | "manage_run" | "manage_schedule" | "manage_factory" | "manage_connections")[];
            /** Format: date-time */
            expires_at: string;
        };
        TokenIssued: {
            /** Format: uuid */
            id: string;
            name: string;
            /** Format: uuid */
            project_id: string;
            scopes: ("read" | "run" | "configure" | "schedule" | "manage_run" | "manage_schedule" | "manage_factory" | "manage_connections")[];
            /** Format: date-time */
            expires_at: string;
            /** Format: date-time */
            revoked_at?: string;
            /** @description 仅此响应返回一次；服务端仅保存摘要。 */
            value: string;
        };
        CatalogPage: {
            items: components["schemas"]["CatalogObject"][];
            next_offset?: number;
        };
        CronPage: {
            items: components["schemas"]["Schedule"][];
            next_offset?: number;
        };
        AuditEntry: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            actor_id: string;
            /** Format: uuid */
            token_id?: string;
            /** Format: uuid */
            request_id: string;
            action: string;
            /** Format: uuid */
            object_id?: string;
            /** Format: uuid */
            project_id?: string;
            /** Format: uuid */
            result_id?: string;
            result_version?: number;
            /** @enum {string} */
            outcome: "pending" | "unknown" | "succeeded" | "rejected" | "failed";
            http_status?: number;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            finished_at?: string;
            /** Format: uuid */
            target_id?: string;
            role?: string;
        };
        AuditPage: {
            items: components["schemas"]["AuditEntry"][];
            next_offset?: number;
        };
        EventPage: {
            items: components["schemas"]["RunEvent"][];
            next_offset?: number;
        };
        RetentionItem: {
            /** Format: uuid */
            run_id: string;
            /** Format: uuid */
            project_id: string;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            expired_at?: string;
            /** Format: date-time */
            purged_at?: string;
            /** @enum {string} */
            status: "protected" | "eligible" | "purged" | "leased" | "failed";
            /**
             * Format: date-time
             * @description 经调度服务确认的终结时间，运行保留期从此起算。
             */
            finished_at?: string;
        };
        RetentionPage: {
            items: components["schemas"]["RetentionItem"][];
        };
        RetentionRecord: {
            id: string;
            /** Format: uuid */
            project_id: string;
            /** Format: uuid */
            run_id: string;
            worker_id: string;
            /** @enum {string} */
            status: "protected" | "purged" | "leased" | "failed";
            /** Format: date-time */
            checked_at: string;
        };
        RetentionHistoryPage: {
            items: components["schemas"]["RetentionRecord"][];
            next_offset?: number;
        };
    };
    responses: {
        /** @description 在实际 Worker 检查环境；Eino configured 不代表凭据或模型可用，未发送收费请求 */
        Preflight: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Preflight"] | components["schemas"]["Error"];
            };
        };
        /** @description 请求或服务错误 */
        Error: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
        /** @description 产品数据库连接/迁移与 Hatchet 编排注册；不检查在线 Worker、模型和工作目录 */
        Readiness: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Readiness"];
            };
        };
    };
    parameters: {
        IdempotencyKey: string;
        ID: string;
    };
    requestBodies: {
        /** @description 请求体最多 256 KiB；字节限制和依赖完整性由服务端校验 */
        Workflow: {
            content: {
                "application/json": components["schemas"]["Workflow"];
            };
        };
    };
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    listFactorySessions: {
        parameters: {
            query: {
                project_id: string;
                offset?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 当前项目的工厂对话 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FactorySessionPage"];
                };
            };
            403: components["responses"]["Error"];
        };
    };
    createFactorySession: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    /** Format: uuid */
                    project_id: string;
                };
            };
        };
        responses: {
            /** @description 已创建空对话，不调用模型 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FactorySession"];
                };
            };
            403: components["responses"]["Error"];
        };
    };
    getFactorySession: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 对话、候选与创建回执；同步已完成生成结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FactorySession"];
                };
            };
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
        };
    };
    generateFactoryProposal: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["FactoryInput"];
            };
        };
        responses: {
            /** @description 单次受限生成任务已保存，消息 ID 用于幂等重试 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Submission"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    applyFactoryProposal: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    /** Format: uuid */
                    turn_id: string;
                    revision: number;
                };
            };
        };
        responses: {
            /** @description 原子创建并发布回执，不运行；同一候选重复请求返回原回执 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FactoryResult"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
        };
    };
    forkRun: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": components["parameters"]["IdempotencyKey"];
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ForkInput"];
            };
        };
        responses: {
            /** @description 基于成功串行 Codex 前缀的独立返工运行 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Submission"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    previewFork: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ForkInput"];
            };
        };
        responses: {
            /** @description 复用与重新执行的影响范围；最终提交还会检查源工作区 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ForkPreview"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    runInputs: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 根据当前已成功上游输出解析参数和分支；未就绪字段不猜测 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["InputPreview"][];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    previewInputs: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Workflow"];
            };
        };
        responses: {
            /** @description 不调用 Agent 的输入预览与错误定位 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["InputPreview"][];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listGithubHooks: {
        parameters: {
            query: {
                project_id: string;
                offset?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 项目管理者可见的事件入口 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["GithubHookPage"];
                };
            };
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    createGithubHook: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["GithubHookConfig"];
            };
        };
        responses: {
            /** @description 固定配置的已启用入口；变更请停用后新建 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["GithubHook"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    disableGithubHook: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 已停用，已接受的提交继续执行 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    githubEvents: {
        parameters: {
            query?: {
                offset?: number;
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 事件与回执追踪 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["GithubEventPage"];
                };
            };
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    receiveGithubIssue: {
        parameters: {
            query?: never;
            header: {
                "X-Hub-Signature-256": string;
                "X-GitHub-Delivery": string;
                "X-GitHub-Event": "issues";
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    [key: string]: unknown;
                };
            };
        };
        responses: {
            /** @description 已在同一事务中记录事件与提交意图，可查询回执 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Submission"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            413: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    runUsage: {
        parameters: {
            query?: {
                offset?: number;
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 原子预留额度及提供方返回的用量；金额只作估算，无账单实值 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["UsageReport"];
                };
            };
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    createDelivery: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": components["parameters"]["IdempotencyKey"];
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
                report: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["DeliveryInput"];
            };
        };
        responses: {
            /** @description 固定交付草案 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Delivery"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listDeliveries: {
        parameters: {
            query: {
                project_id: string;
                offset?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 交付草案列表 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["DeliveryPage"];
                };
            };
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    getDelivery: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 交付草案与运行回执 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Delivery"];
                };
            };
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    runDelivery: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    subject_hash: string;
                };
            };
        };
        responses: {
            /** @description 交付运行回执 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Submission"];
                };
            };
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    exportDelivery: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 已验证、审查并由人采纳的交付 ZIP */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/zip": string;
                };
            };
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listReports: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 报告证据及人工核验 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ReportDetail"][];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    reviewClaim: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
                report: string;
                claim: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ClaimReviewInput"];
            };
        };
        responses: {
            /** @description 报告证据及人工核验 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ClaimReview"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listApprovals: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 运行中的节点审批，最多 32 项；正文随运行清理 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ApprovalGate"][];
                };
            };
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    decideApproval: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ApprovalDecision"];
            };
        };
        responses: {
            /** @description 决策已持久化，事件交接会重试；执行前重新检查权限、有效期与 Git 检查点 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ApprovalGate"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    getFeedback: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 获取当前终态运行及产物摘要；运行中的 subject_hash 为空。 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FeedbackState"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    saveFeedback: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器修改请求同时需要正确 Origin；API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["FeedbackInput"];
            };
        };
        responses: {
            /** @description 管理者或运行所有者按修订号保存采纳评价；相同重复请求返回原评价。 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Feedback"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    projectQuality: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 最近 90 天按稳定任务 ID 去重；采纳取任务最新已评定运行，首次采纳仅计算原始运行首次评价。 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Quality"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listEvaluationCases: {
        parameters: {
            query: {
                project_id: string;
                offset?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 固定样本列表，过期或停用正文为空。 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvaluationCasePage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    createEvaluationCase: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器修改请求同时需要正确 Origin；API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EvaluationCaseInput"];
            };
        };
        responses: {
            /** @description 创建固定样本，正文保留 30 天；变更标准必须创建新样本。 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvaluationCase"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    getEvaluationCase: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 固定样本 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvaluationCase"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    retireEvaluationCase: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器修改请求同时需要正确 Origin；API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 停用样本并立即清除正文。 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listEvaluationRuns: {
        parameters: {
            query?: {
                offset?: number;
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 同一固定样本在多个发布版本的运行与人工采纳结果。 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvaluationRunPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    runEvaluation: {
        parameters: {
            query?: never;
            header: {
                /** @description 浏览器修改请求同时需要正确 Origin；API Token 无此要求。 */
                "X-CSRF-Token"?: string;
                "Idempotency-Key": components["parameters"]["IdempotencyKey"];
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    /** Format: uuid */
                    version_id: string;
                };
            };
        };
        responses: {
            /** @description 使用样本固定输入及指定发布版本运行。保留原请求键重试。 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Submission"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    runnableChoices: {
        parameters: {
            query?: {
                offset?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 可运行助手，每页最多 100 项 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RunnableChoicePage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    codexModelChoices: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 按目录优先级排序的模型候选 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ModelChoicePage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    connectionChoices: {
        parameters: {
            query?: {
                offset?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 可绑定连接版本，每页最多 100 项 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionChoicePage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listProjects: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 当前命名空间已登记的项目 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Project"][];
                };
            };
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    registerProject: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ProjectRegistration"];
            };
        };
        responses: {
            /** @description 项目已登记，执行前在绑定 Worker 校验源目录 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Project"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            413: components["responses"]["Error"];
            415: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    getHealth: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 进程存活 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Health"];
                };
            };
        };
    };
    getReadiness: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            200: components["responses"]["Readiness"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Readiness"];
        };
    };
    getContract: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 当前 API 契约 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/yaml": string;
                };
            };
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
        };
    };
    validateWorkflow: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: components["requestBodies"]["Workflow"];
        responses: {
            /** @description 仅校验定义，返回稳定分层；不访问模型或调度任务 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Validation"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            413: components["responses"]["Error"];
            415: components["responses"]["Error"];
        };
    };
    listRuns: {
        parameters: {
            query?: {
                offset?: number;
                limit?: number;
                status?: "queued" | "running" | "succeeded" | "failed" | "cancelled" | "cancelling";
                since?: string;
                until?: string;
                /** @description 按项目筛选，同时检查当前身份授权。 */
                project_id?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 当前命名空间的根运行；后续页沿用返回的 since/until，状态仍实时变化 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RunList"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    submitRun: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": components["parameters"]["IdempotencyKey"];
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: components["requestBodies"]["Workflow"];
        responses: {
            /** @description 提交意图已持久化；pending 代表等待交接/对账，accepted 才有 run_id；不表示执行完成 */
            202: {
                headers: {
                    /** @description 提交请求查询地址 */
                    Location?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Submission"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            413: components["responses"]["Error"];
            415: components["responses"]["Error"];
            502: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    getSubmission: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 当前命名空间的持久提交请求 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Submission"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    preflightWorkflow: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: components["requestBodies"]["Workflow"];
        responses: {
            200: components["responses"]["Preflight"];
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            413: components["responses"]["Error"];
            415: components["responses"]["Error"];
            502: components["responses"]["Error"];
            503: components["responses"]["Preflight"];
        };
    };
    cancelRun: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 已受理取消意图；终态运行返回 requested=false；不代表执行器已经停止 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        /** Format: uuid */
                        run_id: string;
                        requested: boolean;
                    };
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    getRun: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 当前命名空间的根运行及直接子节点；刚提交的运行可能暂时未同步 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RunDetails"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listAttempts: {
        parameters: {
            query?: {
                offset?: number;
                limit?: number;
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 尝试摘要；完整日志及原生命令证据通过尝试详情获取 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AttemptPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    getAttempt: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                node: string;
                attempt: number;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 已脱敏的单次执行诊断和原生命令证据；不包含内部推理 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Attempt"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    listArtifacts: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 当前成功检查点到原基准的不可变文件清单 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ArtifactManifest"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    downloadArtifact: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                artifact: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 最大 10 MiB；附件下载，不执行 HTML 或其他活动内容 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                    "text/plain": string;
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    getArtifactDiff: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                artifact: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 最多 1 MiB 的文本差异；二进制只有文件元数据 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        file: components["schemas"]["ArtifactFile"];
                        diff: string;
                        truncated: boolean;
                    };
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    exportArtifacts: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description ZIP 包含新/修改文件及删除清单；最大 100 MiB 原始数据、1000 个产物；遇疑似凭据整包拒绝 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/zip": string;
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    listCrons: {
        parameters: {
            query?: {
                offset?: number;
                /** @description 按项目筛选，同时检查当前身份授权。 */
                project_id?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 按权限过滤；空页仍可能存在 next_offset。 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CronPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    createCron: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    /** @description 显示名称，可含中文和空格；UTF-8 最多128字节，不含控制字符。 */
                    name: string;
                    /** @description UTC 五字段表达式 */
                    expression: string;
                    workflow: components["schemas"]["Workflow"];
                };
            };
        };
        responses: {
            /** @description 计划已持久登记并注册到 Hatchet */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Schedule"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            413: components["responses"]["Error"];
            415: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    getCron: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 读取计划原始状态 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Schedule"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    editCron: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    revision: number;
                    paused: boolean;
                    cron: {
                        name: string;
                        /** @description UTC 五字段表达式 */
                        expression: string;
                        workflow: components["schemas"]["Workflow"];
                    };
                };
            };
        };
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Schedule"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    deleteCron: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 已删除，不取消已触发运行 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listScheduleFires: {
        parameters: {
            query?: {
                offset?: number;
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 逐次触发与跳过记录，每页最多 100 条 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ScheduleFire"][];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listCatalog: {
        parameters: {
            query?: {
                kind?: string;
                offset?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 按权限过滤；空页仍可能存在 next_offset。 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    createCatalog: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CatalogCreate"];
            };
        };
        responses: {
            /** @description 操作结果 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogObject"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    getCatalog: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogObject"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    saveCatalog: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    revision: number;
                    name: string;
                    spec: components["schemas"]["CatalogSpec"];
                };
            };
        };
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogObject"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    preflightAgentDraft: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AgentTrialInput"];
            };
        };
        responses: {
            /** @description 环境检查结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Preflight"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            503: components["responses"]["Preflight"];
        };
    };
    listAgentTrials: {
        parameters: {
            query: {
                project_id: string;
                offset?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 该角色在指定项目的试运行回执，按创建时间倒序；正文遵循运行保留期限。 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AgentTrialPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
        };
    };
    runAgentDraft: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token"?: string;
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AgentTrialInput"];
            };
        };
        responses: {
            /** @description 已保存试运行意图；即使尚未派发也可查询回执。 */
            202: {
                headers: {
                    /** @description 回执查询地址 */
                    Location?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Submission"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    publishCatalog: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    revision: number;
                };
            };
        };
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogVersion"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    archiveCatalog: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    archived: boolean;
                };
            };
        };
        responses: {
            /** @description 已归档或恢复；历史版本保留 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    catalogVersions: {
        parameters: {
            query?: {
                offset?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogVersion"][];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    catalogVersion: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogVersion"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    runVersion: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    /** Format: uuid */
                    project_id: string;
                    input?: string;
                };
            };
        };
        responses: {
            /** @description 操作结果 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Submission"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    rerun: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Submission"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    cronPreview: {
        parameters: {
            query: {
                expression: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": string[];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    importCatalog: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CatalogDocument"];
            };
        };
        responses: {
            /** @description 操作结果 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogObject"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    exportCatalog: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogDocument"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    exportCatalogVersion: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogDocument"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    getIdentity: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Actor"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    logout: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    listMemberships: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Membership"][];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    listAPITokens: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIToken"][];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    issueAPIToken: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["TokenIssue"];
            };
        };
        responses: {
            /** @description 操作结果 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TokenIssued"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    revokeAPIToken: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    listMembers: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Member"][];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    grantMember: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    open_id: string;
                    /** @enum {string} */
                    role: "manager" | "operator" | "viewer";
                };
            };
        };
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Actor"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    removeMember: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
                user: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    disableUser: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    enableUser: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话修改操作必填，取自 /auth/me；同时必须提供与 PANGU_PUBLIC_URL 完全一致的 Origin。API Token 无此要求。 */
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    feishuLogin: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 设置浏览器状态 Cookie 并跳转飞书 */
            302: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            503: components["responses"]["Error"];
        };
    };
    feishuCallback: {
        parameters: {
            query?: {
                state?: string;
                code?: string;
                error?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 一次性状态和 PKCE 校验后设置会话，跳转站点首页 */
            303: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    listAudit: {
        parameters: {
            query?: {
                project_id?: string;
                offset?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 操作结果 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuditPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    preflightVersion: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    /** Format: uuid */
                    project_id: string;
                    input: string;
                };
            };
        };
        responses: {
            /** @description 配置诊断通过，不代表模型服务调用成功 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Preflight"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            503: components["responses"]["Preflight"];
        };
    };
    runEvents: {
        parameters: {
            query?: {
                node_run_id?: string;
                offset?: number;
                limit?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 事件分页 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EventPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    previewRetention: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 逐项清理状态 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RetentionPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    executeRetention: {
        parameters: {
            query?: never;
            header?: {
                /** @description 浏览器会话写操作必须携带；API Token 请求不适用。 */
                "X-CSRF-Token"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 逐项清理状态 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RetentionPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listRetentionHistory: {
        parameters: {
            query?: {
                offset?: number;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 清理记录 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RetentionHistoryPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    runActivity: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 脱敏后的当前操作，没有活动时为空数组 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TraceEvent"][];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    runTrace: {
        parameters: {
            query?: {
                after?: number;
                before?: number;
                limit?: number;
                node?: string;
                kind?: "node" | "model" | "tool";
                status?: "running" | "succeeded" | "failed" | "cancelled";
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 脱敏后的追踪事件 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TracePage"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    exportTrace: {
        parameters: {
            query?: {
                after?: number;
                before?: number;
                limit?: number;
                node?: string;
                kind?: "node" | "model" | "tool";
                status?: "running" | "succeeded" | "failed" | "cancelled";
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 脱敏后的追踪事件 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/x-ndjson": string;
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            410: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    runDebug: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 持久调试状态和活动节点数 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["DebugState"];
                };
            };
            403: components["responses"]["Error"];
            410: components["responses"]["Error"];
        };
    };
    controlDebug: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    /** @enum {string} */
                    action: "pause" | "resume" | "step";
                    revision: number;
                };
            };
        };
        responses: {
            /** @description 操作后的调试状态 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["DebugState"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
}
