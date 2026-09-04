# `specs use` end to end: the prompt sequence answered on camera, the template's four
# post-use hooks running, and a listing of what landed on disk.
#
# Re-record with: task demo:record:use
# See docs/content/docs/architecture/demo.md before you do — every recording produces
# different bytes, so a GIF that shows nothing new is pure churn.

Output ../static/demo/use.gif

Require specs

Set Shell bash
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Padding 20
Set TypingSpeed 45ms
Set WindowBar Colorful

# 1280px ≈ 122 columns. The widest line the recorded run prints is the ProjectDescription
# default (109 characters); anything narrower wraps it mid-word.
Set Width 1280
# The prompt form is a single huh group of 11 fields — ~47 rows including the help line.
# 1120px ≈ 50 rows, so the whole schema is on screen while it is being answered.
Set Height 1120

Hide
# Scaffold outside the checkout, at a path mounted identically on host and container:
# the post-use hooks start sibling containers whose bind mounts the host resolves.
Type "cd /tmp/specs-demo && rm -rf acme" Enter
# `task` in the generated project resolves git-derived variables, and prints two `fatal:
# not a git repository` lines when nothing above it is a repo. A real user scaffolds
# inside a workspace, so make the scratch directory look like one.
Type "git init --quiet . 2>/dev/null; true" Enter
# The hooks pull ~1GB of images and an npm package on a cold machine. Warm that once,
# off camera, so the recorded run shows the hooks' output instead of progress bars.
Type "specs use specsnl/specs-laravel-project ./acme --use-defaults --yes >/dev/null 2>&1" Enter
Wait@20m
Type "rm -rf acme && clear" Enter
Show

Type "specs use specsnl/specs-laravel-project ./acme" Enter
Sleep 6s

# The form is one group; Enter moves to the next field, Left/Right toggle a confirm.
# AddBugSnag — keep the default No.
Enter
Sleep 1s
# AddE2E — say Yes, so the recording shows a conditional file being written.
Left
Sleep 500ms
Enter
Sleep 1s
# AddFilament — keep No.
Enter
Sleep 1s
# ComposerLicense — MIT is already selected.
Enter
Sleep 1s
# Database — postgres is already selected.
Enter
Sleep 1s
# IssuePrefix — keep ACME.
Enter
Sleep 1s
# PhpVersion — keep 8.5.
Enter
Sleep 1s
# ProjectDescription — keep the default.
Enter
Sleep 1s
# ProjectName — Ctrl+U clears the pre-filled default before typing over it.
Ctrl+U
Type "Acme Storefront"
Sleep 1s
Enter
Sleep 1s
# ProjectShortName — keep acme.
Enter
Sleep 1s
# RepoName — last field, so Enter submits the whole form.
Ctrl+U
Type "specsnl/acme-storefront"
Sleep 1s
Enter

# specs lists the hooks a remote template wants to run and waits for confirmation.
Sleep 3s
Left
Sleep 500ms
Enter

# The four post-use hooks: .env from the example, markdown formatted and linted through
# docker compose, then git init and git add.
Wait@10m
Sleep 2s

Type "ls acme" Enter
Sleep 5s
