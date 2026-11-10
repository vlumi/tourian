# `make help`: lists the `target: ## text` lines of the Makefile, grouped by
# their `##@ Section` headings.
BEGIN { FS = ":.*## " }
function add(line) {
    if (!(sec in body)) order[++n] = sec
    body[sec] = body[sec] line "\n"
}
FNR == 1 { sec = "General" }
/^##@ / { sec = substr($0, 5); next }
/^[a-zA-Z0-9_-]+:.*## / { add(sprintf("  \033[36m%-18s\033[0m %s", $1, $2)) }
END { for (i = 1; i <= n; i++) printf "\n\033[1m%s\033[0m\n%s", order[i], body[order[i]] }
