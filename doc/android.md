export CXX=~/Android/Sdk/ndk/27.0.12077973/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android27-clang++
export CC=~/Android/Sdk/ndk/27.0.12077973/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android27-clang
export ANDROID_SDK_ROOT="~/Android/Sdk/"
gogio -target android -arch arm64 .
