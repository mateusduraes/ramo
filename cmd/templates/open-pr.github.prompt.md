# Open a draft PR for the current worktree

You are running on the host, with the current worktree as your working directory. The GitHub CLI (`gh`) is available and authenticated.

Your task:

1. Inspect `git status` and `git log --oneline origin/main..HEAD` to understand what changed.
2. If there are uncommitted changes, create a logical set of commits with clear messages.
3. Push the current branch to origin (`git push -u origin HEAD`).
4. Open a draft pull request with `gh pr create --draft` using:
   - A concise title summarizing the change.
   - A body that includes: a summary of what changed, a test plan, and (if relevant) a screenshot or verification steps.

When the PR is open successfully, print its URL and exit.
