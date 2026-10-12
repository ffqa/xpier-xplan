/** Brief, Reviews and Intent tabs. */
export const project: Record<string, string> = {
  // Brief: intent card
  Health: "健康",
  Why: "为什么",
  Goal: "目标",
  "Success looks like": "成功的样子",
  "Non-goals": "不做什么",
  Edit: "编辑",
  "Why this exists": "为什么做这个项目",
  "The intent is still empty.": "初衷还是空的。",
  "This project already has history. Let the reviewer reconstruct why it was started, then correct the draft in your own words.":
    "这个项目已经有历史了。让评审工具还原它当初为什么启动，再用你自己的话修改草稿。",
  "Write down why this project exists and what “done” means, in your own words. Drift reviews compare the code against it, and it is the first thing you will read when you come back.":
    "用你自己的话写下这个项目为什么存在、怎样才算“完成”。偏离评审会拿代码和它对照，你回来时最先读到的也是它。",
  "Write it myself": "我自己写",
  "draft — needs your edit": "草稿 — 需要你修改",
  "{reviewer} is drafting the intent…": "{reviewer} 正在起草初衷…",
  "Draft from history": "从历史起草",

  // Brief: park card
  "Park now": "立即停靠",
  "Where you left off": "上次停在哪",
  "No park note yet. Before you switch to another project, press Park and write one line about where you stopped.":
    "还没有停靠便签。切换到其他项目前，点一下“停靠”，用一句话写下你停在了哪里。",
  "Stopped at": "停在",
  "Next step": "下一步",
  "Open questions": "未决问题",
  "Parked at commit {hash}": "停靠于提交 {hash}",

  // Brief: since card
  "Marked as caught up": "已标记为已看完",
  "Recent activity": "最近动态",
  "I'm caught up": "我已看完",
  "Nothing changed since then.": "此后没有任何变化。",
  "Since your last visit": "自上次访问以来",
  "Since your last park note": "自上次停靠以来",
  "Since your {reference}": "自{reference}以来",
  "{count} commit": "{count} 个提交",
  "{count} commits": "{count} 个提交",
  "{count} non-doc file changed.": "{count} 个非文档文件有改动。",
  "{count} non-doc files changed.": "{count} 个非文档文件有改动。",
  "{count} doc changed": "{count} 篇文档有改动",
  "{count} docs changed": "{count} 篇文档有改动",
  "{count} xteam slice moved": "{count} 个 xteam 切片有进展",
  "{count} xteam slices moved": "{count} 个 xteam 切片有进展",
  closed: "已关闭",
  "{count} review finished": "{count} 个评审已完成",
  "{count} reviews finished": "{count} 个评审已完成",
  "No commits found. Park this project once and xplan will show what changed from then on.":
    "没有找到提交。先停靠一次这个项目，之后 xplan 会显示从那时起的变化。",
  "First visit — showing the latest commits. From now on this card shows only what changed since you last looked.":
    "首次访问 — 显示最近的提交。从现在起，这张卡片只显示你上次查看之后的变化。",

  // Brief: reviews card
  "{kind} review queued": "{kind} 评审已排队",
  "External reviews": "外部评审",
  "{count} finding": "{count} 个问题",
  "{count} findings": "{count} 个问题",
  "never run": "从未运行",
  "Run {title}": "运行{title}",

  // Brief: progress and decisions cards
  Progress: "进度",
  "Checklists across docs": "文档中的清单",
  Branch: "分支",
  Uncommitted: "未提交",
  "{count} file": "{count} 个文件",
  "{count} files": "{count} 个文件",
  "Commits 7d / 30d": "提交（7天 / 30天）",
  "Last commit": "最近提交",
  "xteam slices": "xteam 切片",
  "{open} open · {closed} closed": "{open} 进行中 · {closed} 已关闭",
  Changelog: "更新日志",
  "Docs tracked": "已跟踪文档",
  Scanned: "上次扫描",
  never: "从未",
  "Decisions & specs": "决策与规格",

  // Reviews tab
  Answer: "回答",
  Draft: "草稿",
  "Full report": "完整报告",
  "{count} critical": "严重 {count}",
  "{count} high": "高 {count}",
  "{count} medium": "中 {count}",
  "{count} low": "低 {count}",
  "{count} info": "提示 {count}",
  "No reviews yet. Start with Drift: it checks whether the code still serves the intent.":
    "还没有评审。先从 Drift（偏离检查）开始：它会检查代码是否仍在服务于初衷。",
  "No review selected": "未选择评审",
  "Reviews run an agent in read-only mode against the project, with xplan's intent, decisions and progress as context.":
    "评审会以只读模式让代理检查项目，并以 xplan 中的初衷、决策和进度作为上下文。",
  "Asked {reviewer}": "已向 {reviewer} 提问",
  "Ask the reviewer about this project…": "向评审工具询问这个项目…",
  "Cancel requested": "已请求取消",
  "score {score}/100": "评分 {score}/100",
  Cancel: "取消",
  "Open the Intent tab": "打开初衷标签页",
  Reviewer: "评审工具",
  Started: "开始时间",
  Duration: "耗时",
  Trigger: "触发方式",
  "{count} prompt character": "提示词共 {count} 个字符",
  "{count} prompt characters": "提示词共 {count} 个字符",
  "You asked": "你的提问",
  "Waiting for a free review slot…": "正在等待空闲的评审名额…",
  "{reviewer} is reading the project (read-only). This usually takes a few minutes.":
    "{reviewer} 正在阅读项目（只读）。通常需要几分钟。",
  "The project's working tree changed while this review ran. That may be you editing in parallel — or the agent writing despite read-only mode. Check git status.":
    "评审运行期间，项目的工作区发生了变化。可能是你在同时编辑，也可能是代理无视只读模式写入了文件。请检查 git status。",
  Summary: "摘要",
  "Findings · {count}": "问题 · {count}",
  "Suggestion:": "建议：",

  // Intent tab
  "Let the reviewer reconstruct why this project was started, from its README, docs and git history. Read-only; your own text is never overwritten.":
    "让评审工具根据 README、文档和 git 历史，还原这个项目当初为什么启动。只读操作，你自己写的内容永远不会被覆盖。",
  "{reviewer} is reading the README, docs and git history (read-only).":
    "{reviewer} 正在阅读 README、文档和 git 历史（只读）。",
  "Your intent stays as it is; the draft will wait here.":
    "你的初衷保持不变，草稿会在这里等你。",
  "The draft will be filled in here when it is ready.":
    "草稿准备好后会填到这里。",
  "This is an agent's draft, inferred from history. Fix what is wrong, add what only you know (the real why), then delete the line starting with {marker} and save.":
    "这是代理根据历史推断出的草稿。改掉不对的地方，补上只有你知道的内容（真正的原因），然后删除以 {marker} 开头的那一行并保存。",
  "Draft loaded as unsaved changes. Revert brings your text back.":
    "草稿已作为未保存的修改载入。点“还原”可恢复你原来的内容。",
  "Last draft failed: {error}": "上次起草失败：{error}",
  draft: "草稿",
  filled: "已填写",
  empty: "未填写",
  "Saved — still marked as a draft until you delete the Draft line":
    "已保存 — 删除 Draft 那一行之前，仍会标记为草稿",
  "Intent saved": "初衷已保存",
  "Saved — fill in at least Why or Goal so drift reviews have something to compare against":
    "已保存 — 请至少填写 Why（为什么）或 Goal（目标）部分，偏离评审才有可对照的内容",
  "A draft from {reviewer} ({time}) is ready. Your intent was kept.":
    "{reviewer} 的草稿（{time}）已就绪，你的初衷已保留。",
  "Load into editor": "载入编辑器",
  Dismiss: "关闭",
};
