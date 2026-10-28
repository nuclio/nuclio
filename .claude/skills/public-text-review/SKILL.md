---
name: public-text-review
description: Required before publishing any text to the repository - git commit, git push of a new branch, gh pr create/edit/comment/review, tags, releases and release notes. Keeps public text limited to what the code change does, and stops sensitive context from the ticket or the conversation leaking into it.
---

# Public text review

This repository is public. Commits, branch names, tags, PR titles and bodies, review comments, code comments, test names, log and error messages, release notes and the docs changelog are all public. Forks, mirrors and caches keep them even after a rewrite, and release notes are generated from PR titles verbatim.

The main risk is a well-meant explanation. You usually know *why* a change is being made: from the ticket, the conversation, or a report. Writing that reason down to be helpful is exactly what must not happen when the reason is a weakness in the system.

## Step 1: decide whether the change is sensitive

Treat the change as sensitive if **any** of these is true:

- The ticket, the user or the conversation describes it as a security issue, a weakness, or a finding from a review, scan, audit or report.
- It changes authentication, authorization, access checks, session or secret handling, input validation, or what data an endpoint returns.
- It removes or restricts an endpoint, a parameter or a debug feature because it should not be reachable.
- You are unsure. Ask the user.

If the change isn't sensitive, still apply rules 4 and 5 below, then publish as usual.

## Step 2: the title (highest risk)

The PR title and the commit subject are the most exposed strings. The title is public the moment the PR is opened, it's copied into notification emails, every rename stays visible in the PR timeline, and release notes repeat it verbatim. It must be right on the first `gh pr create`; you can't fix it afterwards.

For a sensitive change, build the title mechanically:

- Format: `[<Scope>] <Verb> <code-level object> (<TICKET-ID>)`, for example `[Auth] Refactor permission checks (NUC-1234)` or `[API] Remove unused handlers (NUC-1234)`.
- `<Verb>` is a neutral maintenance verb: Remove, Refactor, Update, Simplify, Validate, Align, Clean up.
- `<code-level object>` names what the diff touches (a module, handler group or setting), never what it protects against or what was possible before.
- Nothing after the ticket ID. No "to prevent ...", "so that ...", "for ...", "fix", "bug" or severity words.
- The commit subject is identical to the title. Pass `--title` explicitly; never use `gh pr create --fill`, which copies whatever the commit says.

Title test, applied on its own: someone who has seen nothing but this one line must not be able to guess what was wrong. If a title answers "why?", it's wrong; it should only answer "what changed?".

## Step 3: write the rest of the public text from the diff alone

For sensitive changes, write every public string as if you had never seen the ticket or the conversation:

1. **Describe only what the code now does**, in plain maintenance terms: "Remove unused handlers", "Validate request size", "Refactor permission checks". Never say what was possible before, who could do it, or what the impact was.
2. **Use the PR template with minimal content.** One line in Description, a short bullet list of code-level changes, and a Testing line. In References, put the ticket ID only (for example `NUC-1234`), with no link text or summary.
3. **Don't signal urgency or category.** Leave out `fix`/`bug` keywords, "important", "urgent" and severity words, even where the repo's title conventions would normally ask for them.
4. **Never mention** reproduction steps, affected versions or deployments, endpoint paths or parameters that show the weakness, payloads, the names of reports, scanners, reporters or customers, or words such as: vulnerability, exploit, attack, attacker, bypass, leak, exposure, disclosure, unauthenticated, privilege, escalation, injection, pentest, CVE, security fix.
5. **No sensitive values, ever**, in any change: no keys, passwords, tokens, connection strings, internal hostnames, cluster names or customer data, including in tests and examples.
6. **Check the diff too, not just the PR text.** Rename or rewrite code comments, docstrings, test names, fixture names, log lines, error messages and branch names that describe the weakness. A test called `test_unauthenticated_user_cannot_read_x` tells the story as clearly as a PR body.
7. **Keep every copy identical.** Use the same neutral wording in the commit, PR title, PR body, every backport PR, cherry-pick message and release note. A backport must not add detail the original left out.
8. **Squash intermediate commits** whose messages say more than the final one.

The full explanation belongs only in the restricted ticket. Public disclosure happens only through a GitHub Security Advisory, after customers are patched, and a maintainer decides when.

## Step 4: independent outsider read

Before publishing sensitive text, start a fresh subagent that has **not** seen the ticket or this conversation. Run it twice: first with **only the proposed title** (no diff, no body), then with the diff plus the exact title, body, commit message and branch name. Each time ask:

> You are reviewing a public open-source change. Based only on this text and diff, what weakness, if any, could someone infer was present before this change? Quote the exact words or code that give it away.

If either run points to anything specific, rewrite that part and run both checks again. The diff alone may still reveal something; that's expected. The goal is that the text adds nothing to it.

## Step 5: approval gate

For sensitive changes, show the user the exact branch name, commit message, PR title and PR body, plus the outsider-read result. Wait for explicit approval before you run `git commit`, `git push` or `gh pr create`/`edit`/`comment`. Approval covers that text only; re-approve after any change.

## If something slipped out

Stop and tell the user right away. Don't try to fix it yourself with edits, force-pushes or deletes: edits keep their history, and cleaning up public history needs a maintainer.
