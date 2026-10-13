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
  '"auto" tries these top to bottom and skips any that is missing or not logged in. Reorder with the arrows, then Save. Edit a reviewer to pick its model, agent or timeout; "Add reviewer" takes any other CLI. Every preset runs the agent read-only.':
    "「auto」从上到下依次尝试，跳过未安装或未登录的工具。用箭头调整顺序后点保存。点编辑可以给工具指定模型、agent 或超时；「添加评审工具」可以接入任何其他命令行工具。所有预设都以只读方式运行。",
  "Add reviewer": "添加评审工具",
  custom: "自定义",
  changed: "已修改",
  "Model, agent, timeout, command": "模型、agent、超时、命令",
  Reset: "恢复默认",
  Delete: "删除",
  "Reset to preset": "恢复为预设",
  "Reset {name} to the preset? Your model, agent and command changes are dropped.":
    "把 {name} 恢复为预设？你改过的模型、agent 和命令都会丢掉。",
  "Delete {name}?": "删除 {name}？",
  "{name} reset": "{name} 已恢复默认",
  "{name} deleted": "{name} 已删除",
  "model {value}": "模型 {value}",
  "agent {value}": "agent {value}",
  "timeout {value}": "超时 {value}",

  // views/reviewer-editor.tsx
  "Edit {name}": "编辑 {name}",
  "Saved {name}": "{name} 已保存",
  "Any agent CLI that can answer a prompt without asking questions works. Keep it read-only: xplan only asks it to review.":
    "任何能直接回答一段提示、中途不提问的 agent 命令行工具都可以。请保持只读：xplan 只让它做评审。",
  "Start from": "从模板开始",
  Blank: "空白",
  "Use 1-40 lowercase letters, digits, dots, dashes or underscores (not auto).":
    "只能用 1-40 个小写字母、数字、点、横线或下划线（不能叫 auto）。",
  "This name is already used.": "这个名称已被使用。",
  Description: "说明",
  "Command (one argument per line)": "命令（每行一个参数）",
  "No shell: each line is passed as is. Placeholders: {prompt} or {prompt_file} (required), {path}, {xplan_home}, {model}, {agent}. A line whose {model} or {agent} is empty is left out.":
    "不经过 shell：每一行原样传给程序。占位符：{prompt} 或 {prompt_file}（必须有一个）、{path}、{xplan_home}、{model}、{agent}。如果某行的 {model} 或 {agent} 为空，这一行会被去掉。",
  Model: "模型",
  Agent: "Agent",
  "e.g. {hint}": "例如 {hint}",
  "List them with: {command}": "查看可用模型：{command}",
  "Empty = the agent's own default": "留空 = 使用工具自己的默认值",
  "Empty = the preset's agent": "留空 = 使用预设的 agent",
  "Add a line with {model} to the command to choose a model":
    "在命令里加一行含 {model} 的参数，就能选择模型",
  "This command has no {agent} slot": "这个命令没有 {agent} 位置",
  "agent default": "工具默认",
  "The agent decides which tools the reviewer gets. Pick one that cannot edit files.":
    "agent 决定评审工具能用哪些工具。请选一个不能修改文件的 agent。",
  Timeout: "超时",
  "Slow models or big projects may need longer, e.g. 40m or 1h. Empty = the global timeout.":
    "模型慢或项目大时可能需要更久，例如 40m 或 1h。留空 = 使用全局超时。",
  "global ({value})": "全局（{value}）",
  "changed from the preset": "已改动预设",
  "Save and test": "保存并测试",
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
