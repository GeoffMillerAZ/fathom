# Agent skills assessment, 2026-10-20

Durable conversion of the owner's Agent Skills Assessment HTML (Claude Code 2.1.304, Codex CLI 0.163.0). Guidance: [tokenomics.md](../../../tokenomics.md). This is a dated assessment, not measured benefit from installing these skills. Repository statistics and usage are the owner's observations; source snapshot SHA-256: `65f88cfc746cfb0491df7935d489ede4224106c5f104429faa230b988b0a9ada`.

Scores are 0 to 5: Impact is value to this setup, Risk is context cost, overlap, license or supply chain (higher is riskier), Proven is independent evidence, Reception is adoption and sentiment. The page groups 16 candidates as Adopt 2, Trial 3 and Pass 11; a grouped candidate can name multiple skills. Stars and install counts measure attention, not quality.

## Findings

- **F1. Focused curated skills help, with smaller coding gains.** [SkillsBench, arXiv 2602.12670](https://arxiv.org/abs/2602.12670), identified as NeurIPS 2026 in the source assessment: curated skills about +16 percentage points, self-generated skills about zero, software engineering about +4.5 points. Two or three focused modules beat comprehensive documentation. The owner's snapshot says 84 tasks; the fetched abstract now describes 86 and still reports negative deltas on 16 of 84. These are benchmark results, not a forecast for this setup.
- **F2. Triggering is the weak link.** [Vercel's Next.js evals](https://vercel.com/blog/agents-md-outperforms-skills-in-our-agent-evals): no skill invocation in 56% of runs; an explicit AGENTS.md pointer raised triggering about 95%. Wording and task context mattered. Every skill relied on needs a short pointer.
- **F3. Listing cost is persistent.** Owner transcript/metadata scan: about 11k description characters per host per session, plus about 9k synced on Claude (about 5k tokens combined). No truncation warning found in those transcripts. That absence does not prove no truncation; descriptions compete in the listing. [Claude listing budget](https://code.claude.com/docs/en/skills#skill-descriptions-are-cut-short), [Codex metadata guidance](https://learn.chatgpt.com/docs/build-skills).
- **F4. Prune by use.** intent-design, cavecrew, impeccable, caveman and intent-* dominate; fathom-feedback and fathom-verify are heavily read on Codex. The owner found seven zero-use skills counting plugin entries and ten duplicated Firecrawl names. See the dated usage table below.
- **F5. Registries carry risk.** [Snyk ToxicSkills, 2026-02-05 corpus](https://snyk.io/blog/toxicskills-malicious-ai-agent-skills-clawhub/): 3,984 registry skills, 13.4% critical, 76 confirmed malicious payloads. The owner's assessment reports vendor repositories clean in that scan; this is not a guarantee about every vendor package or future revision. Copy a pinned commit after reading every file, without an installer skill or automatic update.

## Owner usage, 2026-08-21 to 2026-10-20

Claude counts are Skill calls and slash commands. Codex counts are SKILL.md reads and inflated, so compare relative use. These figures precede the adoption/prune work.

| Skill | Verdict | Claude | Codex | Note |
|---|---|---|---|---|
| intent-design + 7 intent-* | Keep | 8 | 2,218 | Your most used family. doctor, audit and prune overlap in purpose: worth one look. |
| cavecrew, caveman | Keep | 40 | 828 | They route work to the cavecrew agents. AGENTS.md points at both. |
| impeccable | Shorten | 0 | 505 | Heavily used on Codex. At 895 characters, its description is the longest of your own skills and triggers on any frontend task. |
| herdr | Keep | 2 | 17 | Fires only when you name herdr, as intended. |
| firecrawl, -search, -scrape | Keep 3 | 5 | 147 | The firecrawl description is 701 characters: shorten it. |
| firecrawl-agent, -crawl, -download, -map | Prune | 0 | 0 | Zero uses in 60 days. |
| firecrawl-interact, -parse | Prune | 0 | 4 | Near zero. The firecrawl skill covers both. |
| firecrawl plugin, stale 1.0.9 cache | Prune | 0 | 0 | A second listing of all 10 names (6,100 characters). Keep the current 2.0.0 plugin or the user-level copies, not both. |
| find-skills | Prune | 0 | 1 | An installer for third-party skills. See F5. |
| synced claude.ai skills (13) | Review | n/a | n/a | Claude only, 9,122 characters, mostly office documents. Rarely needed in coding sessions. Check the Skills row in `/context`, and switch them off for coding sessions if your version allows it. |
| fathom-* (10 per repo) | Keep | n/a | 505 | fathom-feedback 294 and fathom-verify 164 on Codex. See the fathom section on repository copies. |

## Candidates

Added Fathom-fit lines for Pass candidates are local synthesis from A1-A5, not additional source measurements. For Pass candidates the source records a rejection rationale rather than separate fit, fathom, risk and evidence paragraphs. The rationale below preserves that evidence; Risk remains the original numeric score. No missing measurement is inferred.

### systematic-debugging

Verdict: **Adopt**. [Upstream](https://github.com/obra/superpowers). Scores: Impact 4/5, Risk 1/5, Proven 4/5, Reception 5/5.

A four-phase root-cause method: reproduce, trace back to the cause, test one hypothesis at a time, then fix with a failing test first. No guessing at fixes.

- Fits: Root-cause debugging is your deep-tier work, and troubleshooting is the main thing that clutters the main thread. A fixed method makes it easy to delegate to a critic with a clear brief.
- Fathom: Strong. Its tracing step is exactly fathom's bug-report path: search, then symbol_context, then blast_radius, then read_source. Add a short fathom section to the copy so the evidence comes from the index, and file the root cause as a thread note.
- Install: Copy this one skill from a pinned commit into `~/.agents/skills`, which both hosts read. Read every file first. Do not install the superpowers plugin: it adds a session-start hook and about a dozen more skills.
- Evidence: The most praised single skill in the most-starred collection (302k stars, MIT, updated 2026-10-20). Independent reviews single it out while criticizing the plugin's token cost.

### skill-creator

Verdict: **Adopt**. [Upstream](https://github.com/anthropics/skills). Scores: Impact 3/5, Risk 1/5, Proven 4/5, Reception 5/5.

Anthropic's skill for writing, testing and tightening skills, including evals for whether a skill triggers when it should.

- Fits: You maintain about 30 skills of your own: the intent family, the fathom pack, cavecrew, caveman and herdr. The research says hand-curated skills help and model-written ones don't, so the authoring tool matters more than any new skill.
- Fathom: Use it on the 10 fathom skills first. Codex reads fathom-feedback and fathom-verify heavily, but nothing tests whether Claude triggers them.
- Install: Claude Code only. Codex already ships its own skill-creator in `~/.codex/skills/.system`. Apache-2.0.
- Evidence: anthropics/skills has 182k stars and was updated 2026-10-09. In SkillsBench (NeurIPS 2026), curated skills gained about 16 points on average and self-generated ones gained nothing.

### differential-review and sharp-edges

Verdict: **Trial**. [Upstream](https://github.com/trailofbits/skills). Scores: Impact 3/5, Risk 2/5, Proven 3/5, Reception 3/5.

Security review of a diff by Trail of Bits: what changed, what it touches, and the language traps a reviewer misses.

- Fits: You review a lot of agent-written Go. This adds a security lens that your gates don't have, and it suits a critic-tier spawn.
- Fathom: Compose: start each review from `review_diff` and `blast_radius`, then apply the security reasoning. Findings flow back through fathom's documented SARIF import.
- Risk: CC-BY-SA-4.0, which only matters if you fork and publish.
- Evidence: 7.7k stars, updated 2026-10-20, from the security firm with the best provenance of any source here. No public benchmark.

### gh-fix-ci and gh-address-comments

Verdict: **Trial**. [Upstream](https://github.com/openai/skills). Scores: Impact 3/5, Risk 2/5, Proven 3/5, Reception 3/5.

Fetch a failing CI run or a PR's review comments with `gh`, and work through them in order.

- Fits: They're mechanical loops that a standard-tier worker can own end to end, which keeps CI noise out of the orchestrator.
- Fathom: Neutral. CI logs are not fathom's domain.
- Risk: The repository has no license field. Read before copying, and keep a local copy pinned to a commit.
- Evidence: 28k stars, updated 2026-09-08. Native to Codex, and the format works on Claude Code.

### playwright-cli or agent-browser

Verdict: **Trial**. [Upstream](https://github.com/microsoft/playwright-cli). Scores: Impact 3/5, Risk 2/5, Proven 3/5, Reception 4/5.

A browser that agents drive from the shell: open a page, snapshot it, click, screenshot. It replaces the Playwright MCP server.

- Fits: Your UI checks rely on the Playwright MCP, which failed to connect in this session. A CLI works the same on both hosts and keeps snapshots out of the main context when a worker runs it.
- Fathom: Neutral.
- Pick one: playwright-cli (Microsoft, 14k stars, Apache-2.0) or vercel-labs/agent-browser (44k stars, Apache-2.0). Trial one, on app-kit work.
- Evidence: Both are active, updated in October 2026. agent-browser is the fourth most installed skill on skills.sh.

### superpowers, the plugin

Verdict: **Pass**. [Upstream](https://github.com/obra/superpowers). Scores: Impact 2/5, Risk 3/5, Proven 4/5, Reception 5/5.

- Why not: Its useful parts are adopted: the subagent-driven pattern (AGENTS.md) and systematic-debugging (above). The plugin adds a session-start hook and per-task review overhead that reviewers call token-heavy.
- Would change: You drop agent-hooks or want the whole workflow on one throwaway project.
- Fathom fit: Review and verification compose with fathom; the full workflow overlaps adopted instructions.

### verification-before-completion

Verdict: **Pass**. [Upstream](https://github.com/obra/superpowers). Scores: Impact 2/5, Risk 1/5, Proven 3/5, Reception 4/5.

- Why not: Already covered: AGENTS.md says no completion without evidence, and `fathom verify` proves plan rows.
- Would change: Completion claims without evidence start showing up in reviews.
- Fathom fit: Overlaps fathom verify and evidence-backed completion rules.

### writing-plans, executing-plans, brainstorming

Verdict: **Pass**. [Upstream](https://github.com/obra/superpowers). Scores: Impact 2/5, Risk 1/5, Proven 3/5, Reception 4/5.

- Why not: These overlap your intent-* family and fathom-plan, both of which are in heavy use.
- Would change: You retire the intent practice.
- Fathom fit: Overlaps fathom-plan, plan rows and intent-design.

### tdd

Verdict: **Pass**. [Upstream](https://github.com/mattpocock/skills). Scores: Impact 2/5, Risk 1/5, Proven 3/5, Reception 5/5.

- Why not: Popular (544k installs), but SkillsBench found the smallest gains in software engineering, and test-first is not your stated default.
- Would change: You decide test-first is the default for agent-written code. Then trial this one or superpowers' version, not both.
- Fathom fit: Outside fathom's evidence role; no new integration justified.

### grill-with-docs, to-spec

Verdict: **Pass**. [Upstream](https://github.com/mattpocock/skills). Scores: Impact 2/5, Risk 1/5, Proven 3/5, Reception 5/5.

- Why not: Top-10 installs, but they do what intent-design and your spec-driven practice already do.
- Would change: A project without the intent system needs a fast spec interview.
- Fathom fit: Overlaps intent-design and spec practice; no new fathom surface.

### frontend-design

Verdict: **Pass**. [Upstream](https://github.com/anthropics/skills). Scores: Impact 2/5, Risk 1/5, Proven 3/5, Reception 5/5.

- Why not: Impeccable already owns UI critique and is heavily used on Codex. Two UI skills compete for the same trigger.
- Would change: You drop impeccable.
- Fathom fit: Outside fathom's role; impeccable already handles visual work.

### cc-skills-golang

Verdict: **Pass**. [Upstream](https://github.com/samber/cc-skills-golang). Scores: Impact 2/5, Risk 2/5, Proven 2/5, Reception 3/5.

- Why not: The only Go collection with real adoption (3.5k stars), but its opinions pull against your stdlib-first, CUE-owns-shape style.
- Would change: You cherry-pick one skill after reading it against agentic-kit's practices.
- Fathom fit: No new evidence integration; reconcile any selected method with kit practices.

### planning-with-files

Verdict: **Pass**. [Upstream](https://github.com/OthmanAdi/planning-with-files). Scores: Impact 2/5, Risk 2/5, Proven 3/5, Reception 4/5.

- Why not: Fathom thread notes and plan rows already persist plans across /clear and compaction, and this one re-injects its plan every turn.
- Would change: Fathom isn't wired in a repo where you run long sessions.
- Fathom fit: Overlaps thread notes and persistent plan rows.

### Agent-Skills-for-Context-Engineering

Verdict: **Pass**. [Upstream](https://github.com/muratcankoylan/Agent-Skills-for-Context-Engineering). Scores: Impact 1/5, Risk 1/5, Proven 2/5, Reception 4/5.

- Why not: Reference reading, not a workflow. docs/tokenomics.md is your version, measured on your own sessions.
- Would change: Never as a skill. Read it once.
- Fathom fit: Reference material for tokenomics, no new fathom tool.

### find-skills

Verdict: **Pass**. [Upstream](https://github.com/vercel-labs/skills). Scores: Impact 1/5, Risk 4/5, Proven 3/5, Reception 5/5.

- Why not: The most installed skill anywhere (2.7M), but it is an installer: it fetches and installs third-party skills, which is the supply-chain risk Snyk found in 13% of registry skills. You have it installed and used it once.
- Would change: Never. Install by hand from a pinned commit.
- Fathom fit: Installer behavior is outside fathom; pinned-copy practice preferred.

### Nix skills

Verdict: **Pass**. [Upstream](https://github.com/papercomputeco/flake-skills). Scores: Impact 1/5, Risk 2/5, Proven 1/5, Reception 1/5.

- Why not: None has real adoption (12 to 62 stars). Your repo's CLAUDE.md rules are better and specific to your setup.
- Would change: One reaches real adoption with independent praise.
- Fathom fit: Machine-specific instructions belong outside fathom.

## Fathom fit

- **A1** Repository skill copies: fix in fathom. In 17 repos, including workstation-repo, `.agents/skills` is a second real copy of `.claude/skills`, so Codex and Claude can drift apart. Your global rule says it should be a link, and only 4 repos follow it. The fathom pack is what fills these folders, so `fathom init --skills` should create the link, and fathom's reconcile step should repair existing repos.
- **A2** Debugging with evidence: compose, don't add a tool. systematic-debugging provides the method and fathom provides the evidence: search, symbol_context, blast_radius, read_source, and then a thread note for the root cause. Keep the skill in agentic-kit with a short fathom section. Fathom's rule against net tool growth argues against a new fathom tool for this.
- **A3** Skill trigger readings: a candidate for fathom metrics. Fathom already measures sessions offline from local transcripts. Which skills fire, on which host, and how much of the listing budget they use is the same kind of measurement. It would tell fathom whether its own 10 skills trigger. File it as a future, not a build.
- **A4** Security review: compose through findings. Trail of Bits' review reasoning starts from fathom's diff impact, and its findings return through the SARIF import fathom already plans. Detection stays with the skill.
- **A5** Not for fathom: browser driving, CI repair, TDD and document skills. They sit outside fathom's evidence role.

A1-A5 are recommendations from the assessment, not completed fathom changes. In particular A3 remains a future, not a build. The later workstation deployment is the source of truth for local installation location.

## Decision

Owner chose on 2026-10-20 to adopt systematic-debugging and skill-creator (Claude only; Codex ships its own), trial differential-review and sharp-edges, and prune the zero-use Firecrawl sub-skills, stale Firecrawl plugin listing and find-skills. Pinned installs and exact local changes are recorded in the workstation repository's [files/agents/skills/PROVENANCE.md](https://github.com/GeoffMillerAZ/workstation/blob/main/files/agents/skills/PROVENANCE.md).

Implementation qualification, 2026-10-20: systematic-debugging and the two review skills were installed. Anthropic skill-creator was rejected under the owner's script safety boundary: external asset fetches, writes outside cwd, and an unrelated-listener kill in the viewer. Its adoption remains blocked pending a safe reviewed adaptation; the existing synced Claude skill and Codex's bundled skill are distinct implementations. Firecrawl 2.0.0 was retained for its hosted MCP and research skills; Claude's duplicate user-level Firecrawl entry was archived. No claim of improved engineering outcomes follows from installation.
