---
title: njulogin
excerpt: 南京大学 p.nju.edu.cn 校园网认证的命令行客户端，本机能用，SSH 能到的机器也能用。
weight: 25
presentation:
  category: 校园网络 / CLI · macOS + Linux
  label: 用 Homebrew 安装
  command:
    - brew trust --tap soundadam/tap
    - brew install --cask soundadam/tap/njulogin
  note: 远端主机什么都不用装。没有账号，没有遥测。
registry:
  github: soundadam/njulogin
  homebrew: soundadam/tap/njulogin
  docs: https://github.com/soundadam/njulogin#readme
release:
  version: "0.2.0"
  license: MIT
modules:
  - type: promo
    description: "不开浏览器登录南大校园网。`njulogin login` 用保存的密码登录，`njulogin qr` 在终端里画出统一身份认证的二维码，用南京大学 App 扫码即可；`status`、`logout`、`kick` 覆盖认证页上其余的操作。"
    line: "{title} {version} 是 {license} 开源的命令行工具，支持 macOS 和 Linux（Intel 与 ARM）。"
    actions:
      - label: 文档
        url: "{docs}"
      - label: GitHub
        url: "https://github.com/{github}"

  - type: snapshot
    title: 为什么做 njulogin
    body: "`p.nju.edu.cn` 的认证页是给浏览器用的。笔记本上还好，实验室服务器、NAS、插在校园网口上的无头机器就麻烦：不先登录，它什么都访问不了。{title} 直接说认证页的协议，登录是一条命令，脚本看退出码就知道成没成；只能 SSH 上去的机器，用的也是同一条命令。"
    note: "与南京大学无隶属关系。保存的密码是一个明文文件，靠 Unix 权限保护（目录 `0700`、文件 `0600`），不进钥匙串。"
    metrics:
      - value: "0"
        label: 远端除 sshd 外需要的工具
      - value: "2"
        label: 登录方式：保存的密码或统一认证二维码
      - value: "4"
        label: 发布构建：macOS 与 Linux，各有 x86_64 和 ARM
      - value: "{version}"
        label: "当前版本 · {license}"

  - type: section
    title: 给只能 SSH 上去的机器登录
    index: 1/2
    lede: "`njulogin remote my-host login` 登录 `my-host` 背后的那条网络。远端不执行任何东西：本机程序经它开 `ssh -W` 隧道直连认证页和统一认证，HTTP、TLS、cookie、二维码轮询都在本机完成。SSH 别名、密钥、ProxyJump 照常生效；`remote my-host qr` 会把那台机器的二维码显示在你自己的屏幕上。"

  - type: section
    title: 密码不进命令行参数，也不进日志
    index: 2/2
    lede: "`njulogin setup` 不回显地读入密码。一次登录只取一个凭据来源：保存的文件、`--ask`、`--password-stdin`，或交给可信密钥助手的 `--password-command`。输出会脱敏，失败返回非零退出码；`njulogin update` 负责升级，并按 GitHub 公布的摘要校验下载。"
---
