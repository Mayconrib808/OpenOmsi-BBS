# Toolchain for the Android build (sourced by scripts/build-android.sh).
: "${ANDROID_HOME:=${ANDROID_SDK_ROOT:-/opt/homebrew/share/android-commandlinetools}}"
if [ -z "${ANDROID_NDK_HOME:-}" ]; then
    ANDROID_NDK_HOME=$(ls -d "$ANDROID_HOME"/ndk/* 2>/dev/null | sort -V | tail -1)
fi
: "${ANDROID_API:=26}"
if [ -z "${JAVA_HOME:-}" ] && [ -x /usr/libexec/java_home ]; then
    JAVA_HOME=$(/usr/libexec/java_home -v 17 2>/dev/null || /usr/libexec/java_home)
fi
case "$(uname -s)" in
    Darwin) HOST_TAG=darwin-x86_64 ;;
    Linux) HOST_TAG=linux-x86_64 ;;
    *) HOST_TAG=windows-x86_64 ;;
esac
NDK_BIN="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/$HOST_TAG/bin"
export ANDROID_HOME ANDROID_NDK_HOME ANDROID_NDK_ROOT="$ANDROID_NDK_HOME" JAVA_HOME
export CC_aarch64_linux_android="$NDK_BIN/aarch64-linux-android$ANDROID_API-clang"
export CXX_aarch64_linux_android="$NDK_BIN/aarch64-linux-android$ANDROID_API-clang++"
export AR_aarch64_linux_android="$NDK_BIN/llvm-ar"
export RANLIB_aarch64_linux_android="$NDK_BIN/llvm-ranlib"
export CARGO_TARGET_AARCH64_LINUX_ANDROID_LINKER="$CC_aarch64_linux_android"
export BUILD_TOOLS=$(ls -d "$ANDROID_HOME"/build-tools/* 2>/dev/null | sort -V | tail -1)
export ANDROID_JAR=$(ls -d "$ANDROID_HOME"/platforms/android-*/android.jar 2>/dev/null | sort -V | tail -1)
# the C++ runtime of the audio backend (oboe) linked in statically: without it the library
# had __cxa_throw & co. unresolved and Android refused to load it (the app closed at once)
export CARGO_TARGET_AARCH64_LINUX_ANDROID_RUSTFLAGS="${CARGO_TARGET_AARCH64_LINUX_ANDROID_RUSTFLAGS:-} -C link-arg=-lc++_static -C link-arg=-lc++abi"
