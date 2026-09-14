#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export JAVA_HOME="$PWD/.local/toolchain/jdk"
export ANDROID_HOME="$PWD/.local/toolchain/sdk"
export ANDROID_SDK_ROOT="$ANDROID_HOME"
export GRADLE_USER_HOME="$PWD/.local/gradle"
export GOPATH="$PWD/.local/gopath"
export GOCACHE="$PWD/.local/gocache"
export GOMAXPROCS=2
export PATH="$PWD/.local/toolchain/go/bin:$JAVA_HOME/bin:$PATH"
exec ./gradlew --no-daemon --max-workers=1 \
  '-Dorg.gradle.jvmargs=-Xmx1024m -Dfile.encoding=UTF-8' \
  -Pkotlin.compiler.execution.strategy=in-process \
  app:assembleMetaRelease -Ptarget.abis=arm64-v8a \
  -Papp.version.name=1.0.0 -Papp.version.code=100000 --console=plain "$@"
