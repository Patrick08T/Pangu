export interface paths {
    "/projects/{id}/experience-policy": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["getExperiencePolicy"];
        put: operations["saveExperiencePolicy"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experience-capabilities": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["getExperienceCapabilities"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experiences": {
        parameters: {
            query?: {
                cursor?: components["parameters"]["ExperienceCursor"];
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listExperiences"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experiences/{version}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        get: operations["getExperience"];
        put?: never;
        post?: never;
        /** @description 不可逆撤销并清正文，保留审计墓碑；不得复活同一版本。 */
        delete: operations["deleteExperience"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experiences/{version}/usages": {
        parameters: {
            query?: {
                cursor?: components["parameters"]["ExperienceCursor"];
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        get: operations["listExperienceUsages"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experiences/{version}/revoke": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["revokeExperience"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experiences/{version}/activate": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description 执行预注册检查器并在激活事务里形成机器回执，禁止传入 Passed 或 receipt。验证正确不等于投放有收益。 */
        post: operations["verifyExperienceClaim"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experiences/{version}/delivery": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["changeExperienceDelivery"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experience-sources": {
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
        post: operations["grantExperienceSource"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experience-sources/{source}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                source: string;
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        delete: operations["withdrawExperienceSource"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experience-checkers": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listExperienceCheckers"];
        put?: never;
        post: operations["registerExperienceChecker"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/evolution/intents": {
        parameters: {
            query?: {
                cursor?: components["parameters"]["ExperienceCursor"];
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listEvolutionIntents"];
        put?: never;
        /** @description 仅提交 Instruction 候选；角色、项目、当前配置和机器证据均由服务端重建。202 不是评测通过。 */
        post: operations["startEvolutionCandidate"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/evolution/intents/{intent}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        get: operations["getEvolutionIntent"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/evolution/intents/{intent}/decision": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["decideEvolutionIntent"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/evolution/intents/{intent}/promote": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["promoteEvolutionIntent"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/evolution/intents/{intent}/observe": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["observeEvolutionIntent"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/experience": {
        parameters: {
            query?: {
                usage_cursor?: string;
                artifact_cursor?: string;
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["runExperienceLineage"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experiences/{version}/observe": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["observeExperienceMemory"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/experience-artifacts/{artifact}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                artifact: string;
            };
            cookie?: never;
        };
        get: operations["experienceArtifactSummary"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/rebuild-intents": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listApprovalRebuildIntents"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/rebuild-intents/{intent}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["rebuildInvalidatedRun"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/evolution/intents/{intent}/rollback": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["rollbackEvolutionIntent"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/evolution/environments": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["listEvolutionEnvironments"];
        put?: never;
        post: operations["registerEvolutionEnvironment"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/entries/{entry}/runs": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                /** @description 工作台入口裸ID，服务端固定workbench种类 */
                entry: string;
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["submitWorkbenchEntry"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/runnables": {
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
        get: operations["projectRunnableChoices"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/versions/{version}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        get: operations["projectCatalogVersion"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{id}/catalog/{object}/versions": {
        parameters: {
            query?: {
                offset?: number;
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                object: string;
            };
            cookie?: never;
        };
        get: operations["projectCatalogVersions"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
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
    "/runs/{id}/evidence": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        get: operations["runEvidence"];
        put?: never;
        post?: never;
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
    "/approvals": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["browseApprovals"];
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
        get: operations["getApproval"];
        /** @description 飞书会话中的运行所有者或管理者批准/驳回；开始执行前可以撤回批准。API Token 不可代替人工审批。 */
        put: operations["decideApproval"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/approvals/{id}/repair": {
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
        /** @description 从修正草稿冻结执行图差异和成功前缀，不发布、不执行；需要运行及配置管理权限。 */
        post: operations["prepareEvidenceRepair"];
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
    "/projects/{id}/workspace": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch: operations["bindPerforceDirectory"];
        trace?: never;
    };
    "/projects/check": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["checkProject"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/runs/{id}/perforce": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        get: operations["perforceDelivery"];
        put?: never;
        post: operations["deliverPerforce"];
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
        ExperienceBudget: {
            /** Format: int64 */
            calls?: number;
            /** Format: int64 */
            tokens?: number;
        };
        ExperienceEvaluationPolicy: {
            suite_version?: string;
            protection_set?: string;
            holdout_set?: string;
            grouping?: string;
            repeats?: number;
            minimum_samples?: number;
            /**
             * @description 启用时只接受 capabilities.supported_metrics 中的真实可测指标
             * @enum {string}
             */
            primary_metric?: "" | "acceptance" | "rework" | "cost" | "duration";
            minimum_improvement?: number;
            non_inferiority?: number | null;
            uncertainty_method?: string;
            confidence?: number;
            max_candidates?: number;
            multiple_comparison?: string;
            resource_tolerance?: number | null;
        };
        ExperienceDeliveryPolicy: {
            /** @enum {string} */
            mode?: "" | "shadow" | "canary" | "serving" | "paused";
            task_scope?: string[] | null;
            traffic_basis_points?: number;
        };
        ExperienceObservationPolicy: {
            /** Format: int64 */
            window_seconds?: number;
            minimum_samples?: number;
            /** Format: int64 */
            deadline_seconds?: number;
            degradation_threshold?: number;
            /** Format: int64 */
            retention_seconds?: number;
            source_retention_authorized?: boolean;
        };
        /** @description 默认关闭。enabled 控制新学习，read_enabled 独立控制已有记忆读取；停学不代表撤销。启用时所有范围、独立预算和评测门槛由服务端校验。 */
        ExperiencePolicySettings: {
            /** @default false */
            enabled: boolean;
            /** @default false */
            read_enabled: boolean;
            /** @default false */
            auto_publish: boolean;
            /** @default false */
            revoked: boolean;
            roles?: string[] | null;
            workflows?: string[] | null;
            entrypoints?: string[] | null;
            extractor_version?: string;
            generator_version?: string;
            environments?: string[] | null;
            learning_budget?: components["schemas"]["ExperienceBudget"];
            evaluation_budget?: components["schemas"]["ExperienceBudget"];
            /** Format: int64 */
            budget_period_seconds?: number;
            /** Format: int64 */
            candidate_interval_seconds?: number;
            evaluation?: components["schemas"]["ExperienceEvaluationPolicy"];
            delivery?: components["schemas"]["ExperienceDeliveryPolicy"];
            observation?: components["schemas"]["ExperienceObservationPolicy"];
        };
        ExperiencePolicy: {
            /** Format: int64 */
            revision: number;
            enabled: boolean;
            read_enabled: boolean;
            auto_publish: boolean;
            revoked: boolean;
            roles: string[] | null;
            workflows: string[] | null;
            entrypoints: string[] | null;
            extractor_version: string;
            generator_version: string;
            environments: string[] | null;
            learning_budget: components["schemas"]["ExperienceBudget"];
            evaluation_budget: components["schemas"]["ExperienceBudget"];
            /** Format: int64 */
            budget_period_seconds: number;
            /** Format: int64 */
            candidate_interval_seconds: number;
            evaluation: components["schemas"]["ExperienceEvaluationPolicy"];
            delivery: components["schemas"]["ExperienceDeliveryPolicy"];
            observation: components["schemas"]["ExperienceObservationPolicy"];
        };
        ExperiencePolicyUpdate: {
            /** Format: int64 */
            expected_revision: number;
            policy: components["schemas"]["ExperiencePolicySettings"];
        };
        ExperienceCapabilities: {
            permissions: {
                manage_policy?: boolean;
                manage_sources?: boolean;
                revoke?: boolean;
                delete?: boolean;
                rollback?: boolean;
                submit_entry?: boolean;
            };
            features: {
                memory_read?: boolean;
                claim_verification?: boolean;
                delivery_promotion?: boolean;
                evolution?: boolean;
                run_lineage?: boolean;
                approval_rebuild?: boolean;
                entry_submission?: boolean;
            };
            supported_metrics: string[];
        };
        ExperienceSummary: {
            /** Format: uuid */
            id: string;
            entry_id: string;
            /** Format: int64 */
            revision: number;
            /** @enum {string} */
            validity: "candidate" | "active" | "superseded" | "revoked" | "expired";
            /** @enum {string} */
            delivery: "shadow" | "canary" | "serving" | "paused";
            /** Format: int64 */
            delivery_revision: number;
            /** Format: date-time */
            expires_at: string;
            /** Format: date-time */
            created_at: string;
        };
        ExperiencePage: {
            items: components["schemas"]["ExperienceSummary"][];
            next_cursor?: string;
        };
        ExperienceContent: {
            kind: string;
            observation: string;
            practice: string;
            conditions: string;
            counterexamples: string;
            applicability: {
                roles: string[];
                role_versions?: string[];
                modules?: string[];
                keywords?: string[];
                error_signatures?: string[];
            };
        };
        ExperienceSource: {
            /** Format: uuid */
            id: string;
            /** @enum {string} */
            kind: "report" | "feedback";
            /** Format: uuid */
            run_id: string;
            node_id: string;
            /** Format: uuid */
            project_id: string;
            hash: string;
        };
        ExperienceVersion: {
            /** Format: uuid */
            id: string;
            entry_id: string;
            /** Format: int64 */
            revision: number;
            /** Format: uuid */
            project_id: string;
            body_hash: string;
            content: components["schemas"]["ExperienceContent"];
            sources: components["schemas"]["ExperienceSource"][];
            /** @enum {string} */
            validity: "candidate" | "active" | "superseded" | "revoked" | "expired";
            /** @enum {string} */
            delivery: "shadow" | "canary" | "serving" | "paused";
            /** Format: int64 */
            delivery_revision: number;
            canary: components["schemas"]["ExperienceDeliveryPolicy"];
            /** Format: date-time */
            expires_at: string;
            tombstoned: boolean;
            invalid_reason: string;
        };
        ExperienceUsage: {
            /** @description 冻结使用身份，可能是提交 request ID；不能当成已验证的 Hatchet run ID 跳转 */
            run_id: string;
            node_id: string;
            iteration: number;
            /** Format: int64 */
            approval_generation: number;
            /** Format: int64 */
            approval_revision: number;
            /** Format: date-time */
            created_at: string;
        };
        ExperienceUsagePage: {
            items: components["schemas"]["ExperienceUsage"][];
            /** @description 不透明的 64 位十六进制游标，不是验收证据 */
            next_cursor?: string;
        };
        ExperienceReason: {
            reason: string;
        };
        ExperienceClaimRequest: {
            /** Format: uuid */
            checker_id: string;
            /** Format: uuid */
            intent_id: string;
            /** Format: int64 */
            generation: number;
        };
        ExperienceDeliveryRequest: {
            /** Format: int64 */
            expected_revision: number;
            policy: components["schemas"]["ExperienceDeliveryPolicy"];
            /**
             * Format: uuid
             * @description 仅定位服务端独立证据，不能提交评分或 Passed
             */
            evidence_id: string;
        };
        ExperienceSourceGrant: {
            /** Format: uuid */
            id: string;
            /** @enum {string} */
            kind: "report" | "feedback";
            /** Format: uuid */
            run_id: string;
            node_id: string;
            hash: string;
            /** Format: date-time */
            expires_at: string;
        };
        ExperienceCheckerSpec: {
            /** @enum {string} */
            kind: "json_assertion_v1";
            content_hash: string;
            /** @enum {string} */
            source_kind: "report" | "feedback";
            node_id: string;
            json_pointer: string;
            expected: boolean | number | string;
        };
        ExperienceCheckerRegistration: {
            spec: components["schemas"]["ExperienceCheckerSpec"];
            /** Format: date-time */
            expires_at: string;
        };
        ExperienceChecker: {
            /** Format: uuid */
            id: string;
            kind: string;
            source_kind: string;
            node_id: string;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            expires_at: string;
        };
        ExperienceCheckerPage: {
            items: components["schemas"]["ExperienceChecker"][];
            next_cursor?: string;
        };
        EvolutionIntentSummary: {
            /** @enum {string} */
            kind?: "role_change" | "memory_benefit";
            /**
             * Format: uuid
             * @description 真实冻结实验标识；不是机器回执或通过证明
             */
            experiment_id?: string;
            /** Format: uuid */
            benefit_memory_version_id?: string;
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            project_id: string;
            /** Format: int64 */
            policy_revision: number;
            generation: number;
            /** @enum {string} */
            state: "preparing" | "evaluating" | "passed" | "publishing" | "switching" | "observing" | "stabilizing" | "rolling_back" | "reconciling" | "stable" | "rolled_back" | "rejected" | "invalidated" | "insufficient";
            publication?: {
                /**
                 * Format: uuid
                 * @description 已发布角色版本 ID
                 */
                agent_id: string;
                /**
                 * Format: uuid
                 * @description 已发布流程版本 ID
                 */
                flow_id?: string;
            };
            /** Format: date-time */
            started_at: string;
        };
        EvolutionIntentPage: {
            items: components["schemas"]["EvolutionIntentSummary"][];
            next_cursor?: string;
        };
        ExperienceReference: {
            /** Format: uuid */
            version_id: string;
            body_hash: string;
            delivery_revision: number;
        };
        ExperienceArtifactReference: {
            id: string;
            hash: string;
        };
        ExperienceAuditState: {
            /** @enum {string} */
            status: "valid" | "invalid" | "unproven" | "restricted";
            proven: boolean;
            invalid_reason?: string;
            redacted: boolean;
        };
        ExperienceWatermarkSummary: components["schemas"]["ExperienceAuditState"] & {
            present: boolean;
            generation: number;
            policy_revision: number;
            hash?: string;
            references: components["schemas"]["ExperienceReference"][] | null;
        };
        ExperienceRunUsageSummary: components["schemas"]["ExperienceAuditState"] & {
            node_id: string;
            iteration: number;
            approval_generation: number;
            approval_revision: number;
            match_reason: string;
            injection_hash?: string;
            references: components["schemas"]["ExperienceReference"][] | null;
            /** Format: date-time */
            created_at: string;
        };
        ExperienceArtifactSummary: components["schemas"]["ExperienceAuditState"] & {
            id: string;
            node_id: string;
            kind: string;
            output_hash?: string;
            input_hash?: string;
            execution_input_hash?: string;
            generation: number;
            references: components["schemas"]["ExperienceReference"][] | null;
            parents: components["schemas"]["ExperienceArtifactReference"][] | null;
        };
        ExperienceRunUsagePage: {
            items: components["schemas"]["ExperienceRunUsageSummary"][];
            total: number;
            has_more: boolean;
            next_cursor?: string;
        };
        ExperienceArtifactPage: {
            items: components["schemas"]["ExperienceArtifactSummary"][];
            total: number;
            has_more: boolean;
            next_cursor?: string;
        };
        RunExperienceLineage: {
            /** Format: uuid */
            actual_run_id: string;
            watermark: components["schemas"]["ExperienceWatermarkSummary"];
            usages: components["schemas"]["ExperienceRunUsagePage"];
            artifacts: components["schemas"]["ExperienceArtifactPage"];
            /** @description 分页根成果不等于完整依赖闭包 */
            chain_complete: boolean;
        };
        EvolutionCandidateRequest: {
            /**
             * @default role_change
             * @enum {string}
             */
            kind: "role_change" | "memory_benefit";
            /** Format: uuid */
            benefit_memory_version_id?: string;
            /** Format: uuid */
            id: string;
            policy_revision: number;
            /** Format: uuid */
            baseline_version: string;
            /** Format: uuid */
            environment_id: string;
            /** Format: uuid */
            suite_id: string;
            /** @description 仅role_change可写，memory_benefit保持基线指令不变 */
            instruction?: string;
            /** Format: uuid */
            flow_version?: string;
            binding_ids?: string[];
            memory_version_ids?: string[];
            entries?: {
                /** @enum {string} */
                kind: "workbench" | "cron" | "webhook";
                id: string;
            }[];
            budget: components["schemas"]["ExperienceBudget"];
        } & ({
            /** @enum {string} */
            kind?: "role_change";
            instruction: string;
        } | {
            /** @enum {string} */
            kind: "memory_benefit";
            instruction?: string;
            memory_version_ids?: string[];
        });
        WorkbenchEntrySubmission: {
            input: string;
        };
        ExperienceObservationRequest: {
            /** Format: uuid */
            run_id: string;
        };
        ExperienceObservation: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            version_id: string;
            /** Format: uuid */
            run_id: string;
            /** Format: int64 */
            delivery_revision: number;
            /** Format: date-time */
            finished_at: string;
        };
        EvolutionGeneration: {
            generation: number;
        };
        EvolutionObservationRequest: {
            generation: number;
            /** Format: uuid */
            run_id: string;
        };
        EvolutionDecision: {
            outcome: string;
            category: string;
            decision_hash?: string;
        };
        ApprovalRebuildIntent: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            run_id: string;
            node_id: string;
            from_generation: number;
            to_generation: number;
            reason: string;
            requires_stop: boolean;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            ready_at?: string;
            /** Format: uuid */
            successor_request_id?: string;
            /** @description 当前身份与运行状态允许尝试重建；提交时仍重验基线和代际 */
            can_rebuild: boolean;
            blocked_reason?: string;
        };
        EvolutionRollback: {
            generation: number;
            reason: string;
        };
        EvolutionEnvironment: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            project_id: string;
            /** Format: int64 */
            revision: number;
            /** @enum {string} */
            backend: "eino_json_v1";
            /** Format: uuid */
            connection_version: string;
            enabled: boolean;
            currency: string;
            source_baseline: string;
            config_hash: string;
        };
        EvolutionEnvironmentUpdate: {
            /** Format: uuid */
            id: string;
            /** Format: int64 */
            expected_revision: number;
            /** @enum {string} */
            backend: "eino_json_v1";
            /** Format: uuid */
            connection_version: string;
            enabled: boolean;
        };
        EvolutionEnvironmentPage: {
            items: components["schemas"]["EvolutionEnvironment"][];
            next_cursor?: string;
        };
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
            /** @description 工厂对话的具体 Codex 模型，省略时继承连接或 Worker 默认；不修改候选 Agent 设置 */
            model?: string;
            reasoning_effort?: components["schemas"]["ReasoningEffort"];
            /** @description 省略继承运行环境默认，false 明确关闭，true 开启 */
            fast?: boolean;
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
            loops?: components["schemas"]["Loop"][];
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
            report_nodes?: {
                [key: string]: boolean;
            };
            /** @description 复用研究正文在本次接续快照中的 SHA256，不改变来源历史输出格式 */
            report_hashes?: {
                [key: string]: string;
            };
            reused?: {
                [key: string]: {
                    text: string;
                    run_id: string;
                    report_ids?: string[];
                    report_sha256?: string;
                    iteration?: components["schemas"]["Iteration"];
                    content_revision?: string;
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
            /** @description 新版本接续共同计入预算的来源运行；calls 仅列当前运行明细 */
            budget_sources?: string[];
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
        /** @description 执行前持久等待人工审批，不自动过期。 */
        ApprovalPolicy: {
            /** @description 校验结构化任务映射，审批执行图差异；批准后发布新版本并保留成果续跑 */
            execution_plan?: boolean;
            source_node: string;
            revision_nodes: string[];
        };
        ReviewItem: {
            id: string;
            category: string;
            title: string;
            proposal: string;
            rationale: string;
            evidence_refs: string[];
            task_ids: string[];
            /**
             * @description 有真实备选方案时提供，互斥方案单选，可组合方案多选。
             * @enum {string}
             */
            selection_mode?: "single" | "multiple";
            options?: components["schemas"]["ReviewOption"][];
            /** @description 推荐组合；只作提示，不代表用户选择。 */
            recommended_option_ids?: string[];
            /** @description 当前计划采用的组合；未确定时可为空。与用户选择不同则须修订复审。 */
            planned_option_ids?: string[];
            recommendation_reason?: string;
        };
        ReviewOption: {
            id: string;
            label: string;
            /** @description 方案的收益、成本、风险及适用条件。 */
            description: string;
        };
        ReviewResolution: {
            item_id: string;
            /** @enum {string} */
            status: "addressed" | "unresolved";
            explanation: string;
        };
        Review: {
            title: string;
            summary: string;
            items: components["schemas"]["ReviewItem"][];
            resolutions: components["schemas"]["ReviewResolution"][];
        };
        ReviewAnswer: {
            item_id: string;
            /** @enum {string} */
            decision: "accept" | "revise" | "reject";
            correction: string;
            /** @description 用户明确选择的方案组合；变更当前计划可直接作为纠偏，无需重复输入文本。 */
            selected_option_ids?: string[];
        };
        ApprovalSummary: {
            approval_generation: number;
            invalidation_reason?: string;
            /** Format: date-time */
            invalidated_at?: string;
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            run_id: string;
            node_id: string;
            title: string;
            status: string;
            round: number;
            /** Format: date-time */
            created_at: string;
        };
        ApprovalPage: {
            items: components["schemas"]["ApprovalSummary"][];
            next_offset?: number;
        };
        ApprovalDetails: {
            approval: components["schemas"]["ApprovalGate"];
            run_status: string;
            stopped: boolean;
        };
        ApprovalDecision: {
            /** @description 明确批准此修正方案；与原补证复验互斥，items 和 reason 必须为空 */
            repair_hash?: string;
            subject_hash: string;
            /** @enum {string} */
            decision: "approved" | "changes_requested" | "rejected" | "revoked";
            reason: string;
            items: components["schemas"]["ReviewAnswer"][];
        };
        ApprovalGate: {
            approval_generation: number;
            invalidation_reason?: string;
            /** Format: date-time */
            invalidated_at?: string;
            repair?: components["schemas"]["EvidenceRepair"];
            /** @enum {string} */
            purpose?: "evidence";
            execution?: components["schemas"]["ExecutionPlanPreview"];
            /** Format: uuid */
            inherited_from?: string;
            round: number;
            /** Format: uuid */
            previous_id?: string;
            review?: components["schemas"]["Review"];
            items: components["schemas"]["ReviewAnswer"][];
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
            status: "waiting" | "approved" | "rejected" | "revoked" | "consumed" | "expired" | "changes_requested" | "invalidated";
            actor_id: string;
            reason: string;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            decided_at: string | null;
            /** Format: date-time */
            consumed_at: string | null;
        };
        EvidenceRepairInput: {
            subject_hash: string;
            /** Format: uuid */
            object_id: string;
            revision: number;
            checkpoint_node: string;
            summary: string;
        };
        EvidenceRepair: {
            /** Format: uuid */
            id: string;
            hash: string;
            /** @enum {string} */
            status: "prepared" | "approved" | "consumed";
            summary: string;
            checkpoint_node: string;
            reused_nodes: string[];
            execute_nodes: string[];
            execution: components["schemas"]["ExecutionPlanPreview"];
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
                reasoning_levels?: components["schemas"]["ReasoningEffort"][];
                default_reasoning?: components["schemas"]["ReasoningEffort"];
                fast?: boolean;
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
                /** @enum {string} */
                provider: "codex" | "openai";
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
            /** @enum {string} */
            vcs: "git" | "perforce";
            perforce?: components["schemas"]["PerforceConfig"];
            name: string;
            /** @description Worker 上的本地目录。Git 为仓库路径；Perforce 为已存在 Client 的 Root/AltRoot，任务直接在该根目录的项目子目录执行。 */
            source?: string;
            /** @description 本地分支，不自动 fetch */
            branch?: string;
            worker_id: string;
            /** @description 管理员登记的验证命令参数数组，首项为Worker上的绝对程序路径；每个Codex节点成功后、检查点发布前执行，最长30秒。禁止凭据参数。不配置时为空数组。 */
            verification?: string[];
        } & ({
            /** @enum {unknown} */
            vcs?: "git";
        } | {
            /** @enum {unknown} */
            vcs?: "perforce";
        });
        Project: {
            /** @enum {string} */
            vcs: "git" | "perforce";
            perforce?: components["schemas"]["PerforceConfig"];
            /** Format: uuid */
            id: string;
            name: string;
            source?: string;
            branch?: string;
            worker_id: string;
            /** @description 管理员登记的验证命令参数数组，首项为Worker上的绝对程序路径；每个Codex节点成功后、检查点发布前执行，最长30秒。禁止凭据参数。不配置时为空数组。 */
            verification?: string[];
        };
        PerforceConfig: {
            /** @description host:port 或 ssl:host:port，不接受远程命令协议 */
            port: string;
            user: string;
            /** @description 已存在的本地 Client；运行直接修改绑定工作区，已有 changelist 归属保持不变 */
            client: string;
            /** @description 相对于 Client 映射根的项目子目录；. 表示根目录 */
            scope: string;
        };
        PerforceDelivery: {
            /** @description 绑定的本地 P4 工作区根目录 */
            local_directory?: string;
            /** @description 本轮修改涉及但保持原归属的 changelist（包括 default），不会自动移入或 shelve 到本轮清单 */
            existing_changelists?: string[];
            /** Format: int64 */
            baseline: number;
            client: string;
            checkpoint: string;
            /** Format: int64 */
            changelist?: number;
            /** @enum {string} */
            status: "available" | "preparing" | "shelved" | "failed" | "empty";
            error?: string;
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
        EvidenceManifest: {
            /** Format: uuid */
            run_id: string;
            /** Format: uuid */
            project_id: string;
            /** Format: date-time */
            collected_at: string;
            missing_sources: string[];
            limitations: string[];
            files: {
                path: string;
                /** Format: uuid */
                run_id: string;
                kind: string;
                title: string;
                sha256: string;
                bytes: number;
            }[];
        };
        BusinessOutcome: {
            /** @enum {string} */
            state: "not_assessed" | "pending" | "accepted" | "needs_attention" | "continued";
            nodes: {
                node_id: string;
                /** @enum {string} */
                state: "not_assessed" | "accepted" | "needs_attention";
                summary?: string;
            }[];
        };
        RunSummary: {
            outcome?: components["schemas"]["BusinessOutcome"];
            /** @description 从任务输入提取的简短标题；过期后不再返回。 */
            title?: string;
            /** Format: uuid */
            id: string;
            /** @enum {string} */
            status: "queued" | "waiting_approval" | "revising_plan" | "running" | "cancelling" | "succeeded" | "failed" | "cancelled" | "skipped" | "unknown";
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
            loop_id?: string;
            /** @description 业务修复轮次，与模型重试和执行尝试分别记录 */
            loop_round?: number;
            /** Format: uuid */
            reused_from?: string;
            review_round?: number;
            /** @description 当前节点执行所在的模型重试序号，首次执行为 0；普通节点尝试另计。 */
            model_retry?: number;
            /**
             * Format: date-time
             * @description 最近一次临时模型故障的最早重试时间。仅活动运行中等待重试的节点返回，取消或终止后清除。
             */
            next_retry_at?: string;
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
            successor?: components["schemas"]["Submission"];
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
            loops?: components["schemas"]["Loop"][];
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
        /** @description 平台控制的循环块，块内无环、不重叠、不嵌套；质量门全部通过后才离开，不设轮数上限。 */
        Loop: {
            id: string;
            nodes: string[];
            /** @description 依次执行的只读质量门，输出必填 passed 布尔值与 summary 文本 */
            gates: string[];
        };
        Iteration: {
            id: string;
            round: number;
            gate?: boolean;
            feedback?: {
                round: number;
                node_id: string;
                node_run_id: string;
                summary: string;
                report_ids?: string[];
            };
        };
        /** @enum {string} */
        ReasoningEffort: "none" | "minimal" | "low" | "medium" | "high" | "xhigh" | "max" | "ultra";
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
            reasoning_effort?: components["schemas"]["ReasoningEffort"];
            /** @description Codex Fast 模式；省略沿用运行环境，false 显式使用普通档位。自定义运行选项必须选择具体模型。 */
            fast?: boolean;
            depends_on?: string[];
            /** Format: uuid */
            agent_version?: string;
            /** Format: uuid */
            connection_version?: string;
            output_schema?: components["schemas"]["Shape"];
            report_schema?: components["schemas"]["Shape"];
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
            report_schema?: components["schemas"]["Shape"];
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
        ExecutionPlanPreview: {
            /** Format: uuid */
            base_version: string;
            /** Format: uuid */
            object_id: string;
            revision: number;
            flow: components["schemas"]["Flow"];
            workflow: components["schemas"]["Workflow"];
            changes: {
                node_id: string;
                before?: components["schemas"]["FlowBinding"];
                after?: components["schemas"]["FlowBinding"];
            }[];
            assignments: {
                task_id: string;
                node_id: string;
            }[];
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
            loops?: components["schemas"]["Loop"][];
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
            /**
             * Format: date-time
             * @description 最近保存、首次发布该修订或归档状态变更的时间。
             */
            updated_at: string;
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
        ExperienceCSRF: string;
        ExperienceVersionID: string;
        EvolutionIntentID: string;
        ExperienceCursor: string;
        ExperienceLimit: number;
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
    getExperiencePolicy: {
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
            /** @description 默认关闭或当前项目策略 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperiencePolicy"];
                };
            };
            403: components["responses"]["Error"];
        };
    };
    saveExperiencePolicy: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExperiencePolicyUpdate"];
            };
        };
        responses: {
            /** @description 原库 CAS 保存后的新修订 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperiencePolicy"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
        };
    };
    getExperienceCapabilities: {
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
            /** @description 实时权限及已接通能力；缺省项不得视为授权 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperienceCapabilities"];
                };
            };
            403: components["responses"]["Error"];
        };
    };
    listExperiences: {
        parameters: {
            query?: {
                cursor?: components["parameters"]["ExperienceCursor"];
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 同事务重新检查来源授权后的元数据页 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperiencePage"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            410: components["responses"]["Error"];
        };
    };
    getExperience: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 读取原证据重新授权后的经验；非保护或保留集内容 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperienceVersion"];
                };
            };
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            410: components["responses"]["Error"];
        };
    };
    deleteExperience: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExperienceReason"];
            };
        };
        responses: {
            /** @description 已撤销并清正文 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
        };
    };
    listExperienceUsages: {
        parameters: {
            query?: {
                cursor?: components["parameters"]["ExperienceCursor"];
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 使用关系摘要；检索或引用不等于成功归因，不返回检索文本及原始注入 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperienceUsagePage"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
        };
    };
    revokeExperience: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExperienceReason"];
            };
        };
        responses: {
            /** @description 已幂等撤销；旧来源与传递成果不得继续复用 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
        };
    };
    verifyExperienceClaim: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExperienceClaimRequest"];
            };
        };
        responses: {
            /** @description 经真实检查激活为 shadow */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperienceVersion"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            410: components["responses"]["Error"];
        };
    };
    changeExperienceDelivery: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExperienceDeliveryRequest"];
            };
        };
        responses: {
            /** @description 服务端独立收益核验后的投放状态 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperienceVersion"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    grantExperienceSource: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExperienceSourceGrant"];
            };
        };
        responses: {
            /** @description 针对原始真实证据的有限保留授权；已曝光历史不可洗白为隐藏集 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
        };
    };
    withdrawExperienceSource: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                source: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 撤销来源保留授权，不抹除已曝光历史 */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
        };
    };
    listExperienceCheckers: {
        parameters: {
            query?: {
                cursor?: components["parameters"]["ExperienceCursor"];
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 检查器元数据，不返回 expected 或可作答案探测的内容哈希 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperienceCheckerPage"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
        };
    };
    registerExperienceChecker: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExperienceCheckerRegistration"];
            };
        };
        responses: {
            /** @description 预注册窄范围事实检查器，不可用于已有候选的事后验收 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperienceChecker"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
        };
    };
    listEvolutionIntents: {
        parameters: {
            query?: {
                cursor?: components["parameters"]["ExperienceCursor"];
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 管理员只读历史；策略停用或未接执行器不隐藏历史 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvolutionIntentPage"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
        };
    };
    startEvolutionCandidate: {
        parameters: {
            query?: {
                cursor?: components["parameters"]["ExperienceCursor"];
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EvolutionCandidateRequest"];
            };
        };
        responses: {
            /** @description 持久接收独立实验 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvolutionIntentSummary"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            422: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    getEvolutionIntent: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 安全的代际与发布状态；不暴露隐藏套件、机器结果或逐例反馈 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvolutionIntentSummary"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
        };
    };
    decideEvolutionIntent: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EvolutionGeneration"];
            };
        };
        responses: {
            /** @description 重读真实机器证据后的最小判定摘要，无逐例反馈或区间 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvolutionDecision"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    promoteEvolutionIntent: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EvolutionGeneration"];
            };
        };
        responses: {
            /** @description 持久发布状态，不代表入口已完成远端切换 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvolutionIntentSummary"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    observeEvolutionIntent: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EvolutionObservationRequest"];
            };
        };
        responses: {
            /** @description 从实际业务运行重读的观察状态，不接收分数或通过标志 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvolutionIntentSummary"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    runExperienceLineage: {
        parameters: {
            query?: {
                usage_cursor?: string;
                artifact_cursor?: string;
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 由真实运行与请求绑定推导的经验水位及分页依赖摘要；旧任务缺水位标记 unproven */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RunExperienceLineage"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    observeExperienceMemory: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExperienceObservationRequest"];
            };
        };
        responses: {
            /** @description 原始业务反馈和真实投放使用记录派生的观察；不接受调用方结果 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperienceObservation"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
        };
    };
    experienceArtifactSummary: {
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
            /** @description 显式展开同项目父依赖；不返回成果正文；来源受限返回 redacted 摘要 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExperienceArtifactSummary"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    listApprovalRebuildIntents: {
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
            /** @description 失效代际和独立后继请求，不恢复旧审批 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ApprovalRebuildIntent"][];
                };
            };
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
        };
    };
    rebuildInvalidatedRun: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": Record<string, never>;
            };
        };
        responses: {
            /** @description 原创建人获准的幂等后继请求 */
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
    rollbackEvolutionIntent: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
                intent: components["parameters"]["EvolutionIntentID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EvolutionRollback"];
            };
        };
        responses: {
            /** @description 已接收回退意图；不代表远端已完成，不撤销既有外部副作用 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvolutionIntentSummary"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    listEvolutionEnvironments: {
        parameters: {
            query?: {
                cursor?: components["parameters"]["ExperienceCursor"];
                limit?: components["parameters"]["ExperienceLimit"];
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 项目内最新环境修订，含停用环境 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvolutionEnvironmentPage"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
        };
    };
    registerEvolutionEnvironment: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
            };
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EvolutionEnvironmentUpdate"];
            };
        };
        responses: {
            /** @description CAS 注册真实无工具受控环境；哈希与源基线由服务端计算 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvolutionEnvironment"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            409: components["responses"]["Error"];
            422: components["responses"]["Error"];
        };
    };
    submitWorkbenchEntry: {
        parameters: {
            query?: never;
            header: {
                "X-CSRF-Token": components["parameters"]["ExperienceCSRF"];
                "Idempotency-Key": string;
            };
            path: {
                id: components["parameters"]["ID"];
                /** @description 工作台入口裸ID，服务端固定workbench种类 */
                entry: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["WorkbenchEntrySubmission"];
            };
        };
        responses: {
            /** @description 真实入口绑定已核验并产生持久业务请求；pending不代表执行完成 */
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
            404: components["responses"]["Error"];
            409: components["responses"]["Error"];
            503: components["responses"]["Error"];
        };
    };
    projectRunnableChoices: {
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
            /** @description 共享手工发布及本项目稳定自动版本；不返回 observing 或墓碑 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RunnableChoicePage"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
        };
    };
    projectCatalogVersion: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                version: components["parameters"]["ExperienceVersionID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 重新检查项目范围和版本可用性 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogVersion"];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
        };
    };
    projectCatalogVersions: {
        parameters: {
            query?: {
                offset?: number;
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
                object: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 项目可用版本，每页100条，按版本号倒序 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogVersion"][];
                };
            };
            400: components["responses"]["Error"];
            403: components["responses"]["Error"];
            404: components["responses"]["Error"];
        };
    };
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
            /** @description 基于成功文件检查点的独立返工运行，保留相同的串行成功前缀及研究报告 */
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
    runEvidence: {
        parameters: {
            query?: {
                /** @description 索引中的精确文件路径，省略则返回索引。 */
                path?: string;
            };
            header?: never;
            path: {
                id: components["parameters"]["ID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 当前原始来源生成的证据索引或指定正文，保持来源运行与覆盖限制。 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvidenceManifest"];
                    "text/plain": string;
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
            /** @description 运行中的所有审批轮次；正文随运行清理 */
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
    browseApprovals: {
        parameters: {
            query: {
                project_id: string;
                run_id?: string;
                node_id?: string;
                status?: "waiting" | "changes_requested" | "approved" | "rejected" | "revoked" | "consumed" | "expired";
                offset?: number;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 项目内最新审核轮次；指定运行后可查询历史轮次 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ApprovalPage"];
                };
            };
            400: components["responses"]["Error"];
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    getApproval: {
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
            /** @description 冻结审核项和当前运行状态 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ApprovalDetails"];
                };
            };
            400: components["responses"]["Error"];
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
            /** @description 决策已持久化；纠偏后修订并复审，批准后重新检查权限、完整报告与 Git 检查点 */
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
    prepareEvidenceRepair: {
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
                "application/json": components["schemas"]["EvidenceRepairInput"];
            };
        };
        responses: {
            /** @description 待用户确认的修正续跑方案 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EvidenceRepair"];
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
    bindPerforceDirectory: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    /** @description Worker 上已有 P4 Client 的 Root 或 AltRoot 绝对路径 */
                    source: string;
                };
            };
        };
        responses: {
            /** @description 已核对本地目录并绑定，后续运行直接在此执行 */
            200: {
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
            502: components["responses"]["Error"];
        };
    };
    checkProject: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ProjectRegistration"];
            };
        };
        responses: {
            /** @description 绑定 Worker 只读检查 Perforce 登录、映射和已提交范围 */
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
            502: components["responses"]["Error"];
        };
    };
    perforceDelivery: {
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
            /** @description Perforce 基线和待审核交付；非 Perforce 运行返回 null */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PerforceDelivery"] | null;
                };
            };
            401: components["responses"]["Error"];
            403: components["responses"]["Error"];
            502: components["responses"]["Error"];
        };
    };
    deliverPerforce: {
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
            /** @description 后台在绑定 Worker 生成或重试同一待审核 changelist，并 shelve；通过 GET 查询进度，不提交 */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PerforceDelivery"];
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
                status?: "queued" | "waiting_approval" | "revising_plan" | "running" | "succeeded" | "failed" | "cancelled" | "cancelling";
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
                /** @description 按归档状态筛选；省略时返回全部，筛选在分页前生效。按最近更新时间倒序排列。 */
                archived?: boolean;
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
            /** @description 校验一次性浏览器状态并通过飞书服务端授权确认身份后，签发绑定个人身份的 Pangu 登录 token；HttpOnly Cookie 与服务端会话固定 30 天过期，访问不续期，跳转站点首页 */
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
                /** @description 仅查询此节点执行的日志，隔离不同循环轮次 */
                node_run_id?: string;
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
                /** @description 仅查询此节点执行的日志，隔离不同循环轮次 */
                node_run_id?: string;
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
