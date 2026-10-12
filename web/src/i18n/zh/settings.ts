/** Global settings, project settings and the status bar. */
export const settings: Record<string, string> = {
  // views/settings-view.tsx: header and language
  "Settings saved": "设置已保存",
  "Stored in": "保存在",
  ". Projects can override reviewer and schedule.":
    "。每个项目可以单独设置评审工具和评审计划。",
  Language: "语言",
  Interface: "界面",
  "Applies at once, also to console replies": "立即生效，控制台回复也会切换",
  Reports: "报告",
  "Language the agent writes reports in": "AI 写评审报告用的语言",
  English: "英文",
  "Simplified Chinese": "简体中文",

  // views/settings-view.tsx: reviews
  "Default reviewer": "默认评审工具",
  "Used by projects that do not pick their own":
    "没有单独指定评审工具的项目使用它",
  "Now: {choice}": "当前：{choice}",
  "Review timeout": "评审超时",
  "Kill a review after this long, e.g. 20m": "超过这个时长就停止评审，例如 20m",
  "Parallel reviews": "同时评审数",
  "Reviews running at the same time (1-8)": "同一时间最多运行几个评审（1-8）",
  "Scan interval": "扫描间隔",
  "How often docs, git and progress are rescanned, e.g. 15m":
    "多久重新扫描一次文档、git 和进度，例如 15m",
  "Default review schedule": "默认评审计划",
  "Copied into newly registered projects; existing projects keep their own schedule (project → Settings). The scheduler starts at most one due review per minute and skips paused and done projects.":
    "新登记的项目会复制这份计划；已有项目保留自己的计划（项目 → 设置）。调度器每分钟最多启动一个到期的评审，并跳过已暂停和已完成的项目。",
  off: "关闭",
  daily: "每天",
  weekly: "每周",
  "Register a folder or discover every git repository under a directory.":
    "登记一个文件夹，或者自动发现某个目录下的所有 git 仓库。",

  // views/settings-view.tsx: reviewer picker
  auto: "auto 自动",
  "first working reviewer": "第一个可用的评审工具",
  "(not installed)": "（未安装）",

  // views/settings-view.tsx: reviewer agents
  "Reviewer agents": "评审工具",
  "Test all": "全部测试",
  Test: "测试",
  "Ask it to reply with a fixed word (read-only)":
    "让它回复一个固定的词（只读）",
  "Try earlier": "往前排",
  "Try later": "往后排",
  "{name} is working": "{name} 可以正常使用",
  "{working} of {total} installed reviewers are working":
    "已安装的 {total} 个评审工具中，{working} 个可以正常使用",
  '"auto" tries these top to bottom and skips any that is missing or not logged in. Reorder with the arrows, then Save. Add or change agents in config.yaml; every preset runs the agent read-only.':
    "「auto」从上到下依次尝试，跳过未安装或未登录的工具。用箭头调整顺序后点保存。在 config.yaml 里增加或修改工具；所有预设都以只读方式运行。",
  "not installed": "未安装",
  "not tested": "未测试",
  "Checked {time} ({source})": "{time}检查（{source}）",
  "during a review": "评审时",
  "connection test": "连接测试",

  // views/project/reviews-tab.tsx: auto picks and job triggers
  "Skipped by auto before picking a reviewer": "auto 选定评审工具前跳过的工具",
  manual: "手动",
  schedule: "定时",
  console: "控制台",
  bulk: "批量",

  // lib/reviewers.ts
  "auto → {name}": "auto → {name}",
  "auto (nothing installed)": "auto（没有已安装的工具）",
  "the first working reviewer": "第一个可用的评审工具",
  "{name} (auto)": "{name}（自动选择）",

  // views/project/settings-tab.tsx
  "Project settings saved": "项目设置已保存",
  "Settings for this project only. Empty values fall back to the global settings.":
    "只对这个项目生效。留空则使用全局设置。",
  General: "基本信息",
  Name: "名称",
  Tags: "标签",
  "Default ({reviewer})": "默认（{reviewer}）",
  "Extra doc globs": "额外的文档匹配规则",
  "One glob per line, relative to the project root. Added on top of the built-in doc detection.":
    "每行一条 glob，相对于项目根目录。会在内置的文档识别之外额外加入。",
  "Danger zone": "危险操作",
  "Unregister removes the project from xplan. The project folder is untouched; xplan's notes move to the archive folder.":
    "取消登记会把项目从 xplan 中移除。项目文件夹不受影响；xplan 的笔记会移到归档文件夹。",
  Unregister: "取消登记",
  "Unregister {name}?": "取消登记 {name}？",
  "The code is not touched. Intent, decisions, journal and reviews are archived, not deleted.":
    "代码不会被改动。初衷、决策、日志和评审都会归档，不会删除。",
  "Keep it": "保留",
  "{name} unregistered. Its notes were moved to the archive folder.":
    "{name} 已取消登记，笔记已移到归档文件夹。",

  // components/status-bar.tsx
  "xplan data directory": "xplan 数据目录",
  "{count} project": "{count} 个项目",
  "{count} projects": "{count} 个项目",
  "{count} need attention": "{count} 个需要关注",
  "API unreachable: {message}": "无法连接 API：{message}",
  "Default reviewer (read-only)": "默认评审工具（只读）",
  "Default reviewer (read-only). Not checked yet: it is tested on first use, or test it in Settings.":
    "默认评审工具（只读）。尚未检查：首次使用时会自动测试，也可以在设置里手动测试。",
  "reports in {language}": "报告语言：{language}",
};
