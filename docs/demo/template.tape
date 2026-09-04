# The registry flow: download a remote template, save a local one, list the registry,
# execute one, then let `specs template update` notice the local source moved ahead.
#
# Two entries rather than one on purpose — the Repository column is what shows a remote
# `owner/repo` and a ~-collapsed local path side by side, and the Status column only means
# something once one of them is out of date.
#
# Re-record with: task demo:record:template
# See docs/content/docs/architecture/demo.md before you do — every recording produces
# different bytes, so a GIF that shows nothing new is pure churn.

Output ../static/demo/template.gif

Require specs

Set Shell bash
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Padding 20
Set TypingSpeed 45ms
Set WindowBar Colorful

# 1000px ≈ 95 columns. The `specs template list` table is 87 columns wide here — the
# Repository column is sized from `specsnl/specs-laravel-project` — and the renderer wraps
# cell contents rather than overflowing, so a narrower frame would fold the table.
Set Width 1000
# 600px ≈ 25 rows: enough for the tallest step — `update` and `list` back to back, both
# tables on screen at once — with the earlier steps scrolling off, as a real session does.
Set Height 600

Hide
# The registry lives under HOME, which is container-local, so every recording starts on an
# empty registry. The scratch directory is not: mirror-mounted from the host, see compose.yml.
Type "cd /tmp/specs-demo && rm -rf minimal billing" Enter
# A committed local template, so `specs template save` records a commit to compare against
# later. The container has no git identity of its own.
Type "export GIT_AUTHOR_NAME='Specs demo' GIT_AUTHOR_EMAIL=demo@specs.nl" Enter
Type "export GIT_COMMITTER_NAME=$GIT_AUTHOR_NAME GIT_COMMITTER_EMAIL=$GIT_AUTHOR_EMAIL" Enter
Type "mkdir -p minimal/template && cd minimal" Enter
Type "printf 'ServiceName: billing\nDescription: A tiny service\n' > project.yml" Enter
Type "printf '# {{ .ServiceName }}\n\n{{ .Description }}\n' > template/README.md" Enter
Type "git init --quiet . && git add . && git commit --quiet -m 'minimal template'" Enter
Type "cd .. && clear" Enter
Show

Type "specs template download specsnl/specs-laravel-project laravel" Enter
Wait@5m
Sleep 2s

Type "specs template save ./minimal minimal" Enter
Wait@1m
Sleep 2s

Type "specs template list" Enter
Wait@1m
Sleep 6s

Type "clear" Enter
Type "specs template use minimal ./billing" Enter
Sleep 4s
# The form prompts in alphabetical order: Description first, then ServiceName.
Enter
Sleep 1s
# ServiceName — last field, so Enter submits.
Enter
Wait@2m
Sleep 2s

Type "cat billing/README.md" Enter
Wait@1m
Sleep 4s

Type "clear" Enter
# Move the saved template's source ahead of what the registry holds.
Type "echo 'Scaffolded with specs.' >> minimal/template/README.md" Enter
Type "git -C minimal commit --quiet -am 'mention specs in the readme'" Enter
Wait@1m
Sleep 2s

Type "specs template update" Enter
Wait@5m
Sleep 5s

Type "specs template list" Enter
Wait@1m
Sleep 8s
