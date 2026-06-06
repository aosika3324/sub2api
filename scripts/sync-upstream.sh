#!/usr/bin/env bash
#
# sync-upstream.sh — 一键把上游 (Wei-Shaw/sub2api) 的更新同步进本 fork
#
# 模型:  main = upstream 的干净镜像;dev = 二次开发集成分支。
# 流程:  fetch upstream → main 快进 → 合并进 dev
#         →(dev 有自定义提交时)重新生成 ent/wire + go mod tidy + pnpm install
#         → 可选测试 → 可选推送
#
# 安全:  工作区不干净 / main 非纯镜像 / 有未完成的 merge 时一律拒绝执行。
#         冲突时停下并给出按文件类型的处理指引;rerere 自动解掉的会继续。
#
# 用法:
#   scripts/sync-upstream.sh                # 同步(不推送、不跑测试)
#   scripts/sync-upstream.sh --push         # 同步后推送 main + dev 到 origin
#   scripts/sync-upstream.sh --test         # 同步后跑后端单元测试 (make test-unit)
#   scripts/sync-upstream.sh --no-generate  # 跳过代码生成
#   scripts/sync-upstream.sh --push --test  # 组合使用
#
# 可用环境变量覆盖:MAIN_BRANCH / DEV_BRANCH / UPSTREAM_REMOTE / ORIGIN_REMOTE
#
set -euo pipefail

# ---------- 可配置项 ----------
MAIN_BRANCH="${MAIN_BRANCH:-main}"
DEV_BRANCH="${DEV_BRANCH:-dev}"
UPSTREAM_REMOTE="${UPSTREAM_REMOTE:-upstream}"
ORIGIN_REMOTE="${ORIGIN_REMOTE:-origin}"

# ---------- 选项 ----------
DO_PUSH=0
DO_TEST=0
DO_GENERATE=1

# ---------- 日志助手 ----------
if [ -t 1 ]; then
  C_RST='\033[0m'; C_INFO='\033[1;34m'; C_OK='\033[1;32m'; C_WARN='\033[1;33m'; C_ERR='\033[1;31m'; C_STEP='\033[1;36m'
else
  C_RST=''; C_INFO=''; C_OK=''; C_WARN=''; C_ERR=''; C_STEP=''
fi
info() { printf "${C_INFO}ℹ %s${C_RST}\n" "$*"; }
ok()   { printf "${C_OK}✓ %s${C_RST}\n" "$*"; }
warn() { printf "${C_WARN}⚠ %s${C_RST}\n" "$*"; }
err()  { printf "${C_ERR}✗ %s${C_RST}\n" "$*" >&2; }
step() { printf "\n${C_STEP}▶ %s${C_RST}\n" "$*"; }
die()  { err "$*"; exit 1; }

usage() {
  cat <<'EOF'
sync-upstream.sh — 一键同步上游到本 fork (main 镜像 / dev 集成)

用法:
  scripts/sync-upstream.sh                # 同步(不推送、不跑测试)
  scripts/sync-upstream.sh --push         # 同步后推送 main + dev 到 origin
  scripts/sync-upstream.sh --test         # 同步后跑后端单元测试 (make test-unit)
  scripts/sync-upstream.sh --no-generate  # 跳过 ent/wire 代码生成
  scripts/sync-upstream.sh --push --test  # 组合使用

环境变量:MAIN_BRANCH / DEV_BRANCH / UPSTREAM_REMOTE / ORIGIN_REMOTE
EOF
  exit 0
}

# ---------- 解析参数 ----------
while [ $# -gt 0 ]; do
  case "$1" in
    --push)        DO_PUSH=1 ;;
    --test)        DO_TEST=1 ;;
    --no-generate) DO_GENERATE=0 ;;
    -h|--help)     usage ;;
    *)             die "未知参数:$1(用 -h 查看用法)" ;;
  esac
  shift
done

# ---------- 定位仓库根 ----------
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || die "不在 git 仓库内"
cd "$REPO_ROOT"
GIT_DIR_PATH="$(git rev-parse --git-dir)"

# ---------- 预检 ----------
step "预检"
git remote get-url "$UPSTREAM_REMOTE" >/dev/null 2>&1 || \
  die "未配置 '$UPSTREAM_REMOTE' remote。先执行:git remote add $UPSTREAM_REMOTE https://github.com/Wei-Shaw/sub2api.git"

[ -e "$GIT_DIR_PATH/MERGE_HEAD" ] && die "有未完成的 merge,先处理:git merge --abort 或解决冲突后提交"
{ [ -d "$GIT_DIR_PATH/rebase-merge" ] || [ -d "$GIT_DIR_PATH/rebase-apply" ]; } && die "有未完成的 rebase,先处理后再同步"

if [ -n "$(git status --porcelain)" ]; then
  err "工作区有未提交改动 —— 同步会切分支,先提交或 stash(这正是上次丢代码的原因):"
  git status --short >&2
  exit 1
fi

git show-ref --verify --quiet "refs/heads/$MAIN_BRANCH" || die "本地没有 '$MAIN_BRANCH' 分支"
git show-ref --verify --quiet "refs/heads/$DEV_BRANCH"  || \
  die "本地没有 '$DEV_BRANCH' 分支(先创建:git checkout $MAIN_BRANCH && git checkout -b $DEV_BRANCH)"

ORIG_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
ok "工作区干净,起始分支:$ORIG_BRANCH"

# ---------- 1. fetch upstream ----------
step "1/5 拉取上游 ($UPSTREAM_REMOTE)"
git fetch "$UPSTREAM_REMOTE" --prune --tags
BEHIND="$(git rev-list --count "$MAIN_BRANCH..$UPSTREAM_REMOTE/$MAIN_BRANCH")"
if [ "$BEHIND" -eq 0 ]; then
  info "$MAIN_BRANCH 已是上游最新,但仍会确保 dev 已并入 main"
else
  info "$MAIN_BRANCH 落后 $UPSTREAM_REMOTE/$MAIN_BRANCH $BEHIND 个提交"
fi

# ---------- 2. main 快进 ----------
step "2/5 $MAIN_BRANCH 快进到 $UPSTREAM_REMOTE/$MAIN_BRANCH"
git checkout "$MAIN_BRANCH"
if ! git merge --ff-only "$UPSTREAM_REMOTE/$MAIN_BRANCH"; then
  err "$MAIN_BRANCH 无法快进 —— 它应当是上游的纯镜像,却出现了自有提交。"
  err "排查:git log $UPSTREAM_REMOTE/$MAIN_BRANCH..$MAIN_BRANCH  (这些提交应当挪到 $DEV_BRANCH)"
  exit 1
fi
ok "$MAIN_BRANCH 已对齐上游"

# ---------- 3. 合并进 dev ----------
step "3/5 把 $MAIN_BRANCH 合并进 $DEV_BRANCH"
git checkout "$DEV_BRANCH"
if git merge --no-edit "$MAIN_BRANCH"; then
  ok "合并完成,无冲突"
else
  UNMERGED="$(git diff --name-only --diff-filter=U)"
  if [ -z "$UNMERGED" ]; then
    warn "rerere 已自动解决全部冲突,提交合并结果"
    git commit --no-edit
    ok "合并完成(rerere 自动解决)"
  else
    err "合并存在冲突,需手动解决以下文件:"
    printf '   %s\n' $UNMERGED >&2
    cat >&2 <<EOF

处理步骤(生成文件别手改,改源头后重新生成):
  • ent 生成代码 backend/ent/**  → 改 backend/ent/schema/*.go 后:  (cd backend && make generate)
  • wire_gen.go                  → 改 cmd/server/wire.go 后:       (cd backend && make generate)
  • go.mod / go.sum             → 取一边后:                       (cd backend && go mod tidy)
  • frontend/pnpm-lock.yaml     → 删冲突文件后:                    pnpm --dir frontend install
  其余冲突手动解决后:
    git add <已解决文件> && git commit
  然后重新运行本脚本(幂等,会补做生成/测试/推送)。
EOF
    exit 1
  fi
fi

# ---------- 4. 重新生成派生产物 ----------
CUSTOM_AFTER="$(git rev-list --count "$UPSTREAM_REMOTE/$MAIN_BRANCH..$DEV_BRANCH")"
if [ "$DO_GENERATE" -eq 0 ]; then
  step "4/5 跳过代码生成(--no-generate)"
elif [ "$CUSTOM_AFTER" -eq 0 ]; then
  step "4/5 $DEV_BRANCH 无超出上游的自定义提交 —— 跳过代码生成"
else
  step "4/5 重新生成 ent/wire + 整理依赖"
  if command -v go >/dev/null 2>&1; then
    ( cd backend && make generate && go mod tidy )
    ok "后端 make generate + go mod tidy 完成"
  else
    warn "未找到 go,跳过后端生成(请自行运行 cd backend && make generate)"
  fi
  if command -v pnpm >/dev/null 2>&1; then
    pnpm --dir frontend install
    ok "前端依赖已同步 (pnpm install)"
  else
    warn "未找到 pnpm,跳过 frontend 依赖同步(若改过 package.json 请手动跑)"
  fi
  if [ -n "$(git status --porcelain)" ]; then
    UP_SHA="$(git rev-parse --short "$UPSTREAM_REMOTE/$MAIN_BRANCH")"
    git add -A
    git commit -m "chore: regenerate after upstream sync ($UP_SHA)"
    ok "已提交重新生成的产物"
  else
    info "生成结果无变化,无需额外提交"
  fi
fi

# ---------- 5. 测试 ----------
if [ "$DO_TEST" -eq 1 ]; then
  step "5/5 后端单元测试 (make test-unit)"
  ( cd backend && make test-unit )
  ok "单元测试通过"
else
  step "5/5 跳过测试(加 --test 可在同步后自动跑 make test-unit)"
fi

# ---------- 推送 ----------
if [ "$DO_PUSH" -eq 1 ]; then
  step "推送到 $ORIGIN_REMOTE"
  git push "$ORIGIN_REMOTE" "$MAIN_BRANCH" || warn "推送 $MAIN_BRANCH 失败(检查 GitHub 凭据)"
  git push "$ORIGIN_REMOTE" "$DEV_BRANCH"  || warn "推送 $DEV_BRANCH 失败(检查 GitHub 凭据)"
fi

# ---------- 收尾 ----------
git checkout "$DEV_BRANCH" >/dev/null 2>&1 || true
step "完成"
ok "$DEV_BRANCH 已同步到上游 $(git rev-parse --short "$UPSTREAM_REMOTE/$MAIN_BRANCH")"
info "$DEV_BRANCH 超出上游的自定义提交数:$(git rev-list --count "$UPSTREAM_REMOTE/$MAIN_BRANCH..$DEV_BRANCH")"
if [ "$DO_PUSH" -eq 0 ]; then
  warn "尚未推送。确认无误后:git push $ORIGIN_REMOTE $MAIN_BRANCH && git push $ORIGIN_REMOTE $DEV_BRANCH"
fi
