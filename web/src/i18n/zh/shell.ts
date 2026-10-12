/** Shell, shared components and formatting helpers. */
export const shell: Record<string, string> = {
  // lib/format.ts: relative time and durations
  now: "刚刚",
  "{n}m": "{n} 分钟",
  "{n}h": "{n} 小时",
  "{n}d": "{n} 天",
  "{n}mo": "{n} 个月",
  "just now": "刚刚",
  "{n}m ago": "{n} 分钟前",
  "{n}h ago": "{n} 小时前",
  "{n}d ago": "{n} 天前",
  "{n}mo ago": "{n} 个月前",
  "{s}s": "{s} 秒",
  "{m}m {s}s": "{m} 分 {s} 秒",

  // lib/format.ts: verdicts
  aligned: "一致",
  drifting: "偏离中",
  "off-track": "已跑偏",
  draft: "草稿",
  answered: "已回答",
  ok: "正常",
  attention: "需关注",
  critical: "严重",
  unknown: "未知",

  // lib/format.ts: review / job kinds
  drift: "偏离",
  code: "代码",
  security: "安全",
  deploy: "部署",
  test: "测试",
  ask: "提问",
  intent: "初衷",

  // lib/format.ts: job statuses
  queued: "排队中",
  running: "运行中",
  done: "完成",
  failed: "失败",
  canceled: "已取消",

  // lib/format.ts: severities
  high: "高",
  medium: "中",
  low: "低",
  info: "提示",

  // lib/format.ts: job titles
  Question: "提问",
  "Intent draft": "初衷草稿",
  "drift review": "偏离评审",
  "code review": "代码评审",
  "security review": "安全评审",
  "deploy review": "部署评审",
  "test review": "测试评审",
  "{kind} review": "{kind} 评审",

  // lib/format.ts: project statuses
  Active: "进行中",
  Waiting: "等待中",
  Blocked: "受阻",
  Paused: "暂停",
  Done: "已完成",
  "You are working on it": "你正在处理",
  "Waiting on an agent, a review or someone else": "在等待代理、评审或其他人",
  "Cannot move until something is resolved": "需要先解决某些问题才能推进",
  "Deliberately parked for later": "有意搁置，留待以后",
  "Shipped or finished": "已上线或已完成",

  // components/status.tsx
  "No issues detected.": "未发现问题。",
  "{kind}: {verdict}": "{kind}：{verdict}",

  // lib/intent-draft.ts
  "Intent draft ready": "初衷草稿已就绪",
  "Intent draft failed": "初衷草稿生成失败",
  "Drafting the intent with {reviewer} (read-only)…":
    "正在用 {reviewer} 起草初衷（只读）…",

  // components/activity-bar.tsx
  "Overview — where every project stands": "总览 — 每个项目的进展",
  "Board — drag projects between states": "看板 — 拖动项目切换状态",
  "Inbox — findings from the latest reviews": "收件箱 — 最近评审发现的问题",
  "Console (⌘J)": "控制台（⌘J）",
  Settings: "设置",

  // components/ui/dialog.tsx
  Close: "关闭",

  // components/add-project-dialog.tsx
  "Registered {name}. Write its intent first — it is what xplan reviews against.":
    "已添加 {name}。请先写下它的初衷——xplan 会以此为依据进行评审。",
  "Registered {count} project": "已添加 {count} 个项目",
  "Registered {count} projects": "已添加 {count} 个项目",
  "Register a project": "添加项目",
  "xplan only reads the project folder. Its own notes live in ~/.xplan, never inside the project.":
    "xplan 只读取项目文件夹，它自己的笔记保存在 ~/.xplan 中，绝不会写入项目内部。",
  "Project folder": "项目文件夹",
  "Display name (optional)": "显示名称（可选）",
  Register: "添加",
  "Or discover git repositories in a folder": "或在某个文件夹中查找 git 仓库",
  Discover: "查找",
  "No unregistered git repositories found there.":
    "该位置没有找到未添加的 git 仓库。",
  "Register {count} selected": "添加所选的 {count} 个",

  // components/command-palette.tsx
  "Go to brief": "前往简报",
  "Go to intent": "前往初衷",
  "Go to decisions": "前往决策",
  "Go to docs": "前往文档",
  "Go to reviews": "前往评审",
  "Go to journal": "前往日志",
  "Go to settings": "前往设置",
  "Search projects, tabs and actions…": "搜索项目、标签页和操作…",
  "Nothing matches “{query}”.": "没有与“{query}”匹配的结果。",
  "This project": "当前项目",
  "Park — leave a note before switching": "停靠 — 切换前留个便签",
  "Run {title}": "运行{title}",
  "Draft intent from history": "根据历史起草初衷",
  "read-only": "只读",
  "{title} queued": "{title}已加入队列",
  Projects: "项目",
  Views: "视图",
  Actions: "操作",
  Overview: "总览",
  Board: "看板",
  Inbox: "收件箱",
  "Toggle console": "切换控制台",
  "Console commands": "控制台命令",

  // components/console-panel.tsx
  Console: "控制台",
  "Agents run read-only. Type /help, a /command, or just ask a question.":
    "代理以只读方式运行。输入 /help、斜杠命令，或直接提问。",
  "Select a project on the left to issue commands.":
    "在左侧选择一个项目以执行命令。",
  "No commands yet. Try {review} to check this project against its intent, or {park} before switching away.":
    "还没有命令。试试用 {review} 对照初衷检查这个项目，或在切换前用 {park} 停靠。",
  "/review drift · /park stopped | next | open · or ask anything about this project":
    "/review drift · /park 停在哪 | 下一步 | 待解决 · 或就这个项目随便提问",
  "{kind} {status} with {reviewer}": "{kind} · {status}（{reviewer}）",
  "{kind} {status}: {error}": "{kind} · {status}：{error}",
  "{kind} {status}": "{kind} · {status}",
  "(empty answer)": "（空回答）",
  "score {score}": "评分 {score}",
  "{count} finding": "{count} 个问题",
  "{count} findings": "{count} 个问题",

  // components/file-viewer.tsx
  Rendered: "渲染视图",
  Source: "源码",
  "xplan never serves secrets (.env, keys, certificates) to the browser.":
    "xplan 绝不会把机密内容（.env、密钥、证书）发送给浏览器。",
  "Binary or oversized file — not shown.": "二进制文件或文件过大 — 不予显示。",

  // components/markdown-editor.tsx
  "Unsaved changes": "未保存的更改",
  Saved: "已保存",
  Edit: "编辑",
  Split: "分栏",
  Preview: "预览",
  Revert: "还原",
  Save: "保存",

  // components/park-dialog.tsx
  "Say where you stopped — that is the part future-you needs most.":
    "写下你停在了哪里——这是将来的你最需要的信息。",
  "Parked {name}. You will see this note first when you come back.":
    "已停靠 {name}。回来时你会最先看到这条便签。",
  "Park {name}": "停靠 {name}",
  "Thirty seconds now saves the warm-up later. This is the first thing you will see when you come back.":
    "现在花 30 秒，省下以后重新热身的时间。回来时你最先看到的就是它。",
  "Where did you stop?": "你停在了哪里？",
  "e.g. OAuth callback works locally; refresh-token rotation half done in TokenService":
    "例如：OAuth 回调在本地已跑通；TokenService 里的 refresh-token 轮换做了一半",
  "Next concrete step": "下一个具体步骤",
  "Last time: {next}": "上次：{next}",
  "e.g. write the rotation test, then wire it into the middleware":
    "例如：先写轮换测试，再接入中间件",
  "Open questions / things not to forget": "待解决问题 / 别忘了的事",
  "e.g. ask whether tokens must survive a deploy":
    "例如：确认 token 在部署后是否必须仍然有效",
  "Last parked {time}": "上次停靠：{time}",
  Park: "停靠",

  // components/project-card.tsx
  "Parked {time}": "停靠于 {time}",
  "next:": "下一步：",
  "No park note yet": "还没有停靠便签",
  "{done}/{total} checklist items done across docs":
    "文档中的清单项已完成 {done}/{total}",
  "Open xteam slices": "未完成的 xteam 切片",
  "Last commit": "最近提交",

  // components/project-sidebar.tsx
  Filter: "筛选",
  "No projects yet.": "还没有项目。",
  "Register one": "添加一个",

  // components/title-bar.tsx
  Brief: "简报",
  Intent: "初衷",
  Decisions: "决策",
  Docs: "文档",
  Reviews: "评审",
  Journal: "日志",
  "Jump to project, tab or command": "跳转到项目、标签页或命令",
  "Toggle console (⌘J)": "切换控制台（⌘J）",
  "{count} running": "{count} 个运行中",
};
