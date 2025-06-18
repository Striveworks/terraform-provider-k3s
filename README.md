<!-- markdownlint-disable -->

# terraform-provider-k3s <picture><source media="(prefers-color-scheme: dark)" srcset="./docs/assets/logo-white.svg" ><img align="right" src="./docs/assets/logo.svg" width="150" /></a></picture>

<!-- markdownlint-restore -->

## Description

Manage your [k3s](https://k3s.io) clusters with terraform. Much of this provider is a reimplementation of [k3s-ansible](https://github.com/k3s-io/k3s-ansible) into terraform
resources so you can manage you k3s clusters together with your cloud provider of choice (or baremetal). This provider is cloud agnostic, but is tested on Openstack.

## Usage

We only guarantee unit testing on the 5 most recent bug minor versions of terraform.

```hcl
terraform {
  required_version = ">= 1.5"

  required_providers {
    k3s = {
      source  = "striveworks/k3s"
      version = ">= 0.0.4"
    }
  }
}
```

## System requirements

[K3S only supports running on modern linux systems](https://docs.k3s.io/installation/requirements#operating-systems).

### K3S Node Support

The provider is tested to target the following cpu and distros

- [x] Ubuntu

with cpu architectures

- [x] amd64

### Provider Host Support

This provider is supported to run on OS-families

- [x] windows
- [x] linux
- [x] darwin

with cpu architectures

- [x] amd64
- [x] arm64

## License

<a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge" alt="License: MIT"></a>

```text
The MIT License (MIT)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

Source: <https://opensource.org/licenses/MIT>
```

## Trademarks

All other trademarks referenced herein are the property of their respective owners.

## Copyrights

Copyright © 2025-2025 [Striveworks](https://www.striveworks.com/)
