# Use a local platform override for Playwright on Ubuntu 20.04

The current development host is detected as Ubuntu 20.04, which the installed Playwright version rejects for Chromium installation. After installing the missing `libgbm1` system dependency, the Ubuntu 22.04 fallback build successfully runs the authentication smoke test.

Document `PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=ubuntu22.04-x64` for both browser installation and test execution on this host. Keep the override local rather than hard-coding it into the shared Playwright configuration; supported hosts should use normal platform detection.
