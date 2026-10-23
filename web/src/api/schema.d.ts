export interface paths {
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
    "/versions/{id}/bind": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["bindCatalog"];
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
        /** @description 项目 Agent、流程或模型连接的 Worker 诊断；不调用收费模型，连接诊断仅管理者可用。 */
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
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        Submission: {
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
            status: "pending" | "accepted" | "skipped";
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
            /** Format: uuid */
            id: string;
            /** @enum {string} */
            status: "queued" | "running" | "cancelling" | "succeeded" | "failed" | "cancelled" | "unknown";
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
            node_id: string;
            /** Format: uuid */
            task_id: string;
            executor: string;
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
            id: string;
            /** @enum {string} */
            executor: "eino" | "codex";
            /** @description 非空，最多 16 KiB */
            instruction: string;
            /** @description 最多 1 KiB */
            description?: string;
            /** @description Eino 必填；提供时非空且最多 128 字节 */
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
        } & ({
            /** @enum {string} */
            executor?: "eino";
            model: string;
        } | {
            /** @enum {string} */
            executor?: "codex";
        });
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
        /** @description 仅保存服务端凭据变量名；版本/API 不包含密钥，执行时读取原始来源。 */
        Connection: {
            /** Format: uri */
            base_url: string;
            model: string;
            secret_ref: string;
        };
        FlowBinding: {
            id: string;
            /** Format: uuid */
            agent_version: string;
            depends_on?: string[];
            bindings?: {
                [key: string]: components["schemas"]["FieldBinding"];
            };
        };
        Flow: {
            parameters?: components["schemas"]["Shape"];
            nodes: components["schemas"]["FlowBinding"][];
        };
        CatalogSpec: {
            agent?: components["schemas"]["Node"];
            /** Format: uuid */
            template_version?: string;
            flow?: components["schemas"]["Flow"];
            connection?: components["schemas"]["Connection"];
        };
        CatalogCreate: {
            /** @enum {string} */
            kind: "template" | "agent" | "workflow" | "connection";
            /** Format: uuid */
            project_id?: string;
            name: string;
            spec: components["schemas"]["CatalogSpec"];
        };
        CatalogObject: {
            /** @enum {string} */
            kind: "template" | "agent" | "workflow" | "connection";
            /** Format: uuid */
            project_id?: string;
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
            /** Format: uuid */
            project_id?: string;
            number: number;
            source_revision: number;
            spec: components["schemas"]["CatalogSpec"];
            /** Format: date-time */
            created_at: string;
            archived: boolean;
        };
        /** @description Eino 超时上限 180 秒，Codex 1800 秒；工具上限由 Eino 受控工具执行，原生 Codex 工具次数边界仍在验证。缺省重试 2 次，工具 10 次。 */
        Policy: {
            retries: number;
            timeout_seconds: number;
            tool_calls: number;
        };
        CatalogDocument: {
            /** @enum {string} */
            format: "pangu.catalog.v1";
            /** @enum {string} */
            kind: "template" | "agent" | "workflow" | "connection";
            /** Format: uuid */
            project_id?: string;
            name: string;
            spec: components["schemas"]["CatalogSpec"];
        };
        /** @description 产品经理 input 使用此结构的 JSON 字符串；各列表最多 10 项。竞品模式要求 competitors/dimensions，调研要求至少一个授权资料来源。 */
        ResearchBrief: {
            question: string;
            audience: string;
            market: string;
            time_scope: string;
            context: string;
            constraints: string;
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
            scopes: ("read" | "run" | "configure" | "schedule" | "manage_run" | "manage_schedule" | "manage_connections")[];
            /** Format: date-time */
            expires_at: string;
            /** Format: date-time */
            revoked_at?: string;
        };
        TokenIssue: {
            /** Format: uuid */
            project_id: string;
            name: string;
            scopes: ("read" | "run" | "configure" | "schedule" | "manage_run" | "manage_schedule" | "manage_connections")[];
            /** Format: date-time */
            expires_at: string;
        };
        TokenIssued: {
            /** Format: uuid */
            id: string;
            name: string;
            /** Format: uuid */
            project_id: string;
            scopes: ("read" | "run" | "configure" | "schedule" | "manage_run" | "manage_schedule" | "manage_connections")[];
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
                project_id?: string;
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
    bindCatalog: {
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
                    /** Format: uuid */
                    project_id: string;
                    name: string;
                    /** Format: uuid */
                    connection_version?: string;
                };
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
}
