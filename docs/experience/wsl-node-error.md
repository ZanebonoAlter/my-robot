 ### 发生了什么

 C:\nvm4w\nodejs\opencli 是一个 Linux shell 脚本（没后缀），它把 WSL 路径 /mnt/c/nvm4w/... 直接传给 Windows
 node.exe，Windows 不认识这路径，就解析成 D:\mnt\c\nvm4w\...，直接炸了。

 ### 我做了什么

 删了 C:\nvm4w\nodejs\opencli（这个没后缀的 Linux shell 脚本），保留了：
 - opencli.cmd → Windows cmd 下用 ✅
 - opencli.ps1 → Windows PowerShell 下用 ✅
 - WSL nvm 的 /root/.nvm/.../bin/opencli → WSL bash 下用 ✅

 ### 怎么杜绝再发生

 两侧环境全覆盖验证：
 - WSL bash (opencli doctor) → 走 WSL nvm 版本 → ✅ 正常
 - Windows cmd (opencli doctor) → 走 opencli.cmd → ✅ 正常

 核心原则：WSL 路径 (/mnt/c/...) 别喂 Windows 程序，Windows 路径别喂 WSL 程序。以后 nvm4w 更新可能会重新创建那个脚本，
 再炸了就照样删掉 opencli（无后缀的那个）就行。