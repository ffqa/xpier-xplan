/** Top-level views and the project frame. */
export const views: Record<string, string> = {
  // Home
  "Drafting {count} intent (read-only).": "正在起草 {count} 份初衷（只读）。",
  "Drafting {count} intents (read-only).": "正在起草 {count} 份初衷（只读）。",
  "{queued} {fresh}": "{queued}{fresh}",
  "{count} new project is left for you to write.":
    "{count} 个新项目留给你自己写。",
  "{count} new projects are left for you to write.":
    "{count} 个新项目留给你自己写。",
  "{count} scan failed": "{count} 次扫描失败",
  "{count} scans failed": "{count} 次扫描失败",
  "Rescanned {count} project": "已重新扫描 {count} 个项目",
  "Rescanned {count} projects": "已重新扫描 {count} 个项目",
  "No projects registered": "还没有登记任何项目",
  "Register the folders you work on. xplan keeps their intent, decisions and progress outside the code, so you can come back without the warm-up.":
    "登记你正在做的项目文件夹。xplan 把它们的初衷、决策和进度保存在代码之外，回来时无需重新热身。",
  "Register projects": "登记项目",
  "Where everything stands": "各项目现状",
  "Projects needing your attention come first. Open one to get its re-entry brief.":
    "需要你关注的项目排在最前。打开任一项目即可查看它的回归简报。",
  "Projects that existed before you registered them, and still have no intent: the reviewer reconstructs why each was started, read-only. You correct the drafts. New projects are left for you to write.":
    "登记之前就已存在、但还没有初衷的项目：评审工具会以只读方式还原每个项目当初为何启动，再由你修正草稿。新项目留给你自己写。",
  "Draft missing intents ({count})": "起草缺失的初衷（{count}）",
  "Rescan all": "全部重新扫描",
  "In flight": "推进中",
  Paused: "暂停",
  Done: "已完成",

  // Inbox
  Inbox: "收件箱",
  "Findings from the latest review of each kind. Re-running a review replaces its findings.":
    "每类评审最近一次发现的问题。重新运行评审会替换它的问题。",
  "All projects": "全部项目",
  "{severity} and above": "{severity}及以上",
  "Inbox zero": "收件箱已清空",
  "Nothing at this severity. Reviews run on each project's schedule, or start one from a project's Reviews tab.":
    "该严重程度下没有问题。评审会按各项目的计划运行，也可以在项目的“评审”标签页手动发起。",
  "Suggestion:": "建议：",
  "Open the full {kind} report": "查看完整的{kind}报告",

  // Project frame
  Brief: "简报",
  Intent: "初衷",
  Decisions: "决策",
  Docs: "文档",
  Reviews: "评审",
  Journal: "日志",
  Settings: "设置",
  "Project not found": "未找到项目",
  "It may have been unregistered.": "它可能已被取消登记。",
  "Back to overview": "返回总览",
  "Intent not written yet": "尚未填写初衷",
  "Moved to {status}": "已移至“{status}”",
  "Scanned {count} doc in {ms}ms": "已扫描 {count} 个文档，耗时 {ms}ms",
  "Scanned {count} docs in {ms}ms": "已扫描 {count} 个文档，耗时 {ms}ms",
  "{kind} review queued with {reviewer}":
    "{kind}评审已排队（评审工具：{reviewer}）",
  "Path copied": "路径已复制",
  "Clipboard unavailable": "剪贴板不可用",
  Healthy: "健康",
  "Copy path": "复制路径",
  "Open in Cursor": "在 Cursor 中打开",
  "Rescan docs, git and progress": "重新扫描文档、git 和进度",
  Park: "停靠",
  Review: "评审",
  "Ask a question in the console": "在控制台中提问",

  // Journal
  All: "全部",
  Note: "笔记",
  Command: "命令",
  System: "系统",
  Timeline: "时间线",
  "Nothing here yet.": "这里还没有内容。",
  Stopped: "停在",
  Next: "下一步",
  Open: "未决",
  "Quick note": "快速笔记",
  "An idea, a gotcha, a link — anything you would otherwise forget.":
    "一个想法、一个坑、一个链接——任何你可能会忘掉的东西。",
  "Add note": "添加笔记",

  // Decisions
  "xplan decisions · {count}": "xplan 决策 · {count}",
  "Record why you chose something, so future-you doesn't re-litigate it.":
    "记下你为什么这样选，免得将来的你再争论一遍。",
  "In the project · {count}": "项目内 · {count}",
  "No ADR or spec files found in the repository.":
    "仓库中没有找到 ADR 或规格文件。",
  "Pick a decision": "选择一条决策",
  "xplan decisions live outside the repository, so they survive branch switches and stay yours. Project ADRs and specs are shown read-only.":
    "xplan 决策保存在仓库之外，切换分支也不会丢失，并且只属于你。项目里的 ADR 和规格以只读方式显示。",
  "New decision title": "新决策标题",
  "Decision saved": "决策已保存",
  "edited {time} · change “Status:” in the text to accept or supersede":
    "{time}编辑 · 在正文中修改“Status:”即可接受或取代",
  "Open in Docs": "在文档中打开",

  // Docs
  Readme: "README",
  "Specs & plans": "规格与计划",
  "Decision records": "决策记录",
  "Progress & tasks": "进度与任务",
  Changelogs: "变更记录",
  "Agent instructions": "Agent 指令",
  "xteam slices": "xteam 切片",
  "Other docs": "其他文档",
  "Docs by kind": "按类型",
  Files: "文件",
  "Pick a document": "选择一个文档",
  "Docs are collected from the project on every scan: READMEs, specs, ADRs, progress files, changelogs, agent rules and xteam slices.":
    "每次扫描都会从项目中收集文档：README、规格、ADR、进度文件、变更记录、Agent 规则和 xteam 切片。",
  "Filter docs": "筛选文档",
  "No docs found.": "没有找到文档。",
  "Doc list truncated — the project has more docs than the scanner keeps.":
    "文档列表已截断——项目中的文档数量超过了扫描器的保留上限。",
  "Sensitive file — never shown": "敏感文件——不会显示",
};
