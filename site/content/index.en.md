---
title: njulogin
excerpt: A command-line client for Nanjing University's p.nju.edu.cn login portal, on this machine or on a host you reach over SSH.
weight: 25
presentation:
  category: Campus network / CLI · macOS + Linux
  label: Install with Homebrew
  command:
    - brew trust --tap soundadam/tap
    - brew install --cask soundadam/tap/njulogin
  note: Nothing to install on the remote host. No account, no telemetry.
registry:
  github: soundadam/njulogin
  homebrew: soundadam/tap/njulogin
  docs: https://github.com/soundadam/njulogin#readme
release:
  version: "0.2.0"
  license: MIT
modules:
  - type: promo
    description: "Log in to NJU's campus network portal without a browser. `njulogin login` uses a saved password, `njulogin qr` draws the CAS QR code in the terminal for the NJU app to scan, and `status`, `logout` and `kick` cover the rest of what the portal page does."
    line: "{title} {version} is an {license}-licensed CLI for macOS and Linux, on Intel and ARM."
    actions:
      - label: Docs
        url: "{docs}"
      - label: GitHub
        url: "https://github.com/{github}"

  - type: snapshot
    title: Why njulogin
    body: "The portal at `p.nju.edu.cn` expects a browser. That is fine on a laptop and awkward on a lab server, a NAS or a headless box behind a campus port that has to log in before it can reach anything. {title} speaks the portal's protocol directly, so a login is one command, a script can check the result from the exit status, and the same command works for a machine you only reach through SSH."
    note: "Not affiliated with Nanjing University. The saved password is a plaintext file protected by Unix permissions (directory `0700`, file `0600`), not a keychain entry."
    metrics:
      - value: "0"
        label: Tools needed on the remote host besides sshd
      - value: "2"
        label: "Ways to log in: saved password or CAS QR code"
      - value: "4"
        label: "Release builds: macOS and Linux, each on x86_64 and ARM"
      - value: "{version}"
        label: "Current release · {license}"

  - type: section
    title: Log in a machine you can only SSH into
    index: 1/2
    lede: "`njulogin remote my-host login` logs in the network path behind `my-host`. Nothing runs on that host: the local binary opens `ssh -W` tunnels through it to the portal and CAS, and handles HTTP, TLS, cookies and QR polling itself. Your SSH aliases, keys and ProxyJump apply as usual, and `remote my-host qr` shows that host's QR code on your own screen."

  - type: section
    title: Passwords stay out of arguments and logs
    index: 2/2
    lede: "`njulogin setup` prompts for the password without echo. A login takes exactly one credential source: the saved file, `--ask`, `--password-stdin`, or `--password-command` for a trusted secret helper. Output is redacted, failures exit nonzero, and `njulogin update` keeps it current, checking each download against the digest GitHub publishes."
---
