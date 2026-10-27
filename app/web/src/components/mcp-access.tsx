import React, { useRef, useState } from "react";
import { Alert, Btn, FormDialog, SelectField, Status } from "./kit";
import { request } from "../lib/api";
import { useRemote, useWorkspace } from "../pages/shared";
import "./mcp-access.css";

type AccessMode = "read_only" | "read_write";
interface Credential {
  id: number;
  tokenPrefix: string;
  accessMode: AccessMode;
  createdAt: string;
  lastUsedAt: string | null;
  revokedAt: string | null;
}
interface Connection {
  credential: Credential | null;
  canWrite: boolean;
  endpoint: string;
}

function localTime(value: string | null) {
  return value
    ? new Date(value).toLocaleString("zh-CN", { timeZone: "Asia/Shanghai" })
    : "尚未调用";
}

export function MCPAccess() {
  const { user, notify } = useWorkspace();
  const info = useRemote(
    () => request<Connection>("GET", "/mcp/credentials"),
    [user.id],
  );
  const [dialog, setDialog] = useState<"create" | "revoke" | null>(null);
  const [mode, setMode] = useState<AccessMode>("read_only");
  const [generated, setGenerated] = useState<string | null>(null);
  const configRef = useRef<HTMLTextAreaElement>(null);
  const credential = info.data?.credential;
  const active = Boolean(credential && !credential.revokedAt);
  const create = () => {
    setMode(
      active && credential?.accessMode === "read_write" && info.data?.canWrite
        ? "read_write"
        : "read_only",
    );
    setDialog("create");
  };
  const copy = async () => {
    if (!generated) return;
    try {
      await navigator.clipboard.writeText(generated);
    } catch {
      configRef.current?.focus();
      configRef.current?.select();
      if (!document.execCommand("copy")) {
        throw new Error("已选中配置，请按 Ctrl/Cmd+C 手动复制后关闭弹窗。");
      }
    }
    notify("Agent 接入配置已复制");
  };
  return (
    <section
      className="panel agent-access"
      aria-labelledby="agent-access-title"
    >
      <div className="section-heading">
        <h2 id="agent-access-title">Agent 接入</h2>
        <Status
          value={active ? "active" : "inactive"}
          label={active ? "已生成连接" : "未连接"}
        />
      </div>
      <p className="agent-access-note">
        配置一次，持续使用。Agent 的工具和数据范围随你的 CC4 账号权限生效。
      </p>
      {info.loading ? (
        <p role="status">正在载入接入状态…</p>
      ) : info.error ? (
        <>
          <Alert>{info.error}</Alert>
          <Btn kind="ghost" onClick={info.reload}>
            重新加载
          </Btn>
        </>
      ) : (
        <>
          {active && credential && (
            <dl className="detail-list">
              <div>
                <dt>凭据标识</dt>
                <dd>{credential.tokenPrefix}…</dd>
              </div>
              <div>
                <dt>接入权限</dt>
                <dd>
                  {credential.accessMode === "read_only"
                    ? "仅查询"
                    : "允许业务写入"}
                </dd>
              </div>
              <div>
                <dt>有效期</dt>
                <dd>长期有效，直到断开或重置</dd>
              </div>
              <div>
                <dt>最近调用</dt>
                <dd>{localTime(credential.lastUsedAt)}</dd>
              </div>
            </dl>
          )}
          <div className="actions">
            <Btn onClick={create}>
              {active ? "重置并生成新配置" : "生成接入配置"}
            </Btn>
            {active && (
              <Btn kind="danger" onClick={() => setDialog("revoke")}>
                断开 Agent
              </Btn>
            )}
            <Btn kind="ghost" onClick={info.reload}>
              刷新状态
            </Btn>
          </div>
          <p className="agent-access-note">
            每个账号只保留一把有效 Token。完整配置仅在生成时展示，丢失后可重置。
          </p>
        </>
      )}
      {dialog === "create" && (
        <FormDialog
          open
          title={active ? "重置 Agent 接入" : "生成 Agent 接入配置"}
          onClose={() => setDialog(null)}
          submitText={active ? "重置并生成" : "生成配置"}
          onSubmit={async () => {
            const result = await request<{ config: unknown }>(
              "POST",
              "/mcp/credentials",
              { label: "个人 Agent", accessMode: mode },
            );
            setGenerated(JSON.stringify(result.config, null, 2));
            info.reload();
          }}
        >
          {active && (
            <Alert>重置后，旧 Token 和原有 Agent 连接将立即失效。</Alert>
          )}
          <SelectField
            label="接入权限"
            value={mode}
            onChange={(event) => setMode(event.target.value as AccessMode)}
          >
            <option value="read_only">仅查询与试算</option>
            <option value="read_write" disabled={!info.data?.canWrite}>
              允许业务写入
            </option>
          </SelectField>
          <p className="agent-access-note">
            {info.data?.canWrite
              ? "允许写入仍受你的业务权限限制，成功代录或审核会修改云端数据。"
              : "你的账号当前可查询授权范围内的数据，没有代录或审核权限。"}
          </p>
        </FormDialog>
      )}
      {dialog === "revoke" && credential && (
        <FormDialog
          open
          title="断开 Agent"
          onClose={() => setDialog(null)}
          submitText="确认断开"
          onSubmit={async () => {
            await request("DELETE", `/mcp/credentials/${credential.id}`);
            info.reload();
            notify("Agent 已断开，原 Token 已失效");
          }}
        >
          <p>当前 Token 将立即失效。需要再次使用时，重新生成接入配置即可。</p>
        </FormDialog>
      )}
      {generated && (
        <FormDialog
          open
          title="复制 Agent 接入配置"
          onClose={() => setGenerated(null)}
          onSubmit={copy}
          submitText="复制配置并关闭"
        >
          <p>
            将配置导入支持 Streamable HTTP 的 Agent。完整 Token
            仅在这次生成时展示。
          </p>
          <label className="field agent-access-config-field">
            <span>Agent 连接配置</span>
            <textarea
              ref={configRef}
              className="agent-access-config"
              aria-label="Agent 连接配置"
              readOnly
              value={generated}
              rows={12}
              spellCheck={false}
            />
          </label>
        </FormDialog>
      )}
    </section>
  );
}
