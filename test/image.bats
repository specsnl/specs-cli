#!/usr/bin/env bats
#
# Acceptance checks for the runtime image, run by both ci.yml and
# `task image:smoke`.
#
# IMAGE and EXPECTED_VERSION name the image under test and the version it must
# report.

setup_file() {
    bats_require_minimum_version 1.5.0

    : "${IMAGE:?set IMAGE to the image reference under test}"
    : "${EXPECTED_VERSION:?set EXPECTED_VERSION to the version the image must report}"

    export TEMPLATE="${BATS_TEST_DIRNAME}/../internal/testdata/hooks"
    export CALLER="$(id -u):$(id -g)"
}

setup() {
    bats_load_library bats-support
    bats_load_library bats-assert
    bats_load_library bats-file

    # Under TMPDIR, which points at a directory mounted at the same path on the
    # host — the daemon resolves the bind mounts below against the host.
    WORKDIR="$(mktemp -d)"
}

teardown() {
    chmod -R u+w "$WORKDIR" 2>/dev/null || true
    rm -rf "$WORKDIR"
}

# Scaffolds into $WORKDIR as the invoking user. HOME is redirected because an
# overridden uid owns no home directory inside the image.
scaffold() {
    docker run --rm \
        --user "$CALLER" \
        --env HOME=/tmp \
        --volume "$TEMPLATE:/template:ro" \
        --volume "$WORKDIR:/work" \
        "$IMAGE" use /template "$@"
}

@test "reports the version injected at build time" {
    run docker run --rm "$IMAGE" --version

    assert_success
    # 'dev' here means SPECS_VERSION never reached the ldflag.
    assert_output "$EXPECTED_VERSION"
}

@test "runs as the non-root specs user" {
    run docker run --rm --entrypoint id "$IMAGE"

    assert_success
    assert_output --partial "uid=1000(specs) gid=1000(specs)"
}

@test "has bash on PATH for template hooks" {
    run docker run --rm --entrypoint bash "$IMAGE" -c 'exit 0'

    assert_success
}

@test "has the hook toolchain on PATH" {
    run docker run --rm --entrypoint bash "$IMAGE" -c \
        'command -v git && command -v task && command -v docker'

    assert_success
}

@test "has the compose plugin wired into the docker CLI" {
    run docker run --rm --entrypoint docker "$IMAGE" compose version

    assert_success
}

@test "git is usable by a foreign uid with HOME redirected" {
    run docker run --rm \
        --user "$CALLER" \
        --env HOME=/tmp \
        --volume "$WORKDIR:/work" \
        --entrypoint bash "$IMAGE" -c 'git init -q . && git status --porcelain'

    assert_success
}

@test "scaffolds into a bind mount and runs the hooks" {
    run scaffold ./out --use-defaults --yes

    assert_success
    assert_file_exist "$WORKDIR/out/main.txt"
    assert_file_exist "$WORKDIR/out/hook-output.txt"
}

@test "writes files owned by the invoking user" {
    scaffold ./out --use-defaults --yes

    run stat -c %u "$WORKDIR/out/main.txt"

    assert_output "$(id -u)"
}

@test "refuses to assume defaults without a terminal" {
    run ! scaffold ./refused

    assert_output --partial "stdin is not a terminal"
}
