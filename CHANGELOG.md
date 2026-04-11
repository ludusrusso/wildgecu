# Changelog

## 1.0.0 (2026-04-11)


### Features

* abstract fs and start writing test ([327c318](https://github.com/ludusrusso/wildgecu/commit/327c31896fab49e986a3682e4192817d5c43105c))
* abstract session manager to have multiple frontend ([4307208](https://github.com/ludusrusso/wildgecu/commit/4307208b881086541060f555b44143fc7ec82cc5))
* add background install capability and an home dir ([de8c06a](https://github.com/ludusrusso/wildgecu/commit/de8c06acbe236c73d846902fb0d9a0423534e954))
* add bash tool ([06fe178](https://github.com/ludusrusso/wildgecu/commit/06fe1789285dfab82a19a9e9eee68540ff114946))
* add cronjobs ([0592f5b](https://github.com/ludusrusso/wildgecu/commit/0592f5b3763bdec0258bdd861330389311325d75))
* add debug ([a436afa](https://github.com/ludusrusso/wildgecu/commit/a436afab6f6298f6ab2c1c19da264d5bc1377206))
* add ESC to interrup ui ([4aa1cdc](https://github.com/ludusrusso/wildgecu/commit/4aa1cdcc7b99674fabb1fc8d7ff5cf1168efad34))
* add inform_user tool for mid-loop progress updates ([680570f](https://github.com/ludusrusso/wildgecu/commit/680570f6047708b259a1b8cc28dcda02a7a60f27))
* add init command, fetch_url tool, and Google Search grounding ([84999a4](https://github.com/ludusrusso/wildgecu/commit/84999a4adeb6a3e159a3126b0404e5224323dd20))
* add makefile ([941df82](https://github.com/ludusrusso/wildgecu/commit/941df825ae0f394faa2c92b9d9df0c383d3918e9))
* add makefiles ([b01a293](https://github.com/ludusrusso/wildgecu/commit/b01a29353bfa5a4c0588db41a10b17b67a6050ca))
* add memory to agent ([7c2cb40](https://github.com/ludusrusso/wildgecu/commit/7c2cb40f54db720c48d2b8a5c68ee6ba9c08591e))
* add OpenAI and Ollama provider support ([#11](https://github.com/ludusrusso/wildgecu/issues/11)) ([50a7c7a](https://github.com/ludusrusso/wildgecu/commit/50a7c7a6d78338e80457cc81a61246214fc7a602))
* add OpenAI-compatible provider constructors for Regolo, Mistral, and Ollama ([294bc7b](https://github.com/ludusrusso/wildgecu/commit/294bc7b664b3c7cda2c59b28cceb0af4b5080f04))
* add parallel tool calling ([1d952b8](https://github.com/ludusrusso/wildgecu/commit/1d952b8eaf7ec90d09faa9129413c4b2421f6475))
* add slash commands with autocomplete for TUI and Telegram ([#23](https://github.com/ludusrusso/wildgecu/issues/23)) ([bc5be7f](https://github.com/ludusrusso/wildgecu/commit/bc5be7f5a5af0921d1e8323c0c1df8c2caa88621))
* add telegram ([99d8f2e](https://github.com/ludusrusso/wildgecu/commit/99d8f2e9567445110c848015ea107d68cc9894e4))
* add telegram user authentication with OTP approval ([ae40b65](https://github.com/ludusrusso/wildgecu/commit/ae40b65bc4fa95b5270ddfd0dc7f1069cbd2dc2e))
* add tests for homer ([d10f2d5](https://github.com/ludusrusso/wildgecu/commit/d10f2d59fbe6b9b8737a3467c0466ef24581c396))
* add tools ([cf01aa6](https://github.com/ludusrusso/wildgecu/commit/cf01aa6e990bea5f0436a5f9502f4822e59e76df))
* cycle philosophical verbs in thinking spinner ([#1](https://github.com/ludusrusso/wildgecu/issues/1)) ([7c51317](https://github.com/ludusrusso/wildgecu/commit/7c51317ce5b640fd421fa0748f07fad115a16104))
* first commit, wellcome gonesis ([6c5ace4](https://github.com/ludusrusso/wildgecu/commit/6c5ace425d6b55b2b6de38637e83aabab4b8ee54))
* implement code mode with file-system tools and specialized prompt ([ffab863](https://github.com/ludusrusso/wildgecu/commit/ffab8632a91233519d08a43e123013979bfd1faa))
* implement inform and tool call callbacks in telegram bridge ([c8a36e7](https://github.com/ludusrusso/wildgecu/commit/c8a36e741464217a4abb71e965c2c95e5787bf60))
* implement skills ([727818f](https://github.com/ludusrusso/wildgecu/commit/727818f7dde2bb9848d35ae8a2c68de91be55d15))
* implement tools tests ([ade5179](https://github.com/ludusrusso/wildgecu/commit/ade5179a1eb98d6720cd5a121ce88a711d9586fe))
* improve Agent.md ([95c56e6](https://github.com/ludusrusso/wildgecu/commit/95c56e65777d83e7a6ff96813f5f0668c0fdb912))
* multi-provider config with lazy instantiation ([#32](https://github.com/ludusrusso/wildgecu/issues/32)) ([1da98d1](https://github.com/ludusrusso/wildgecu/commit/1da98d1b4c5792b8dd03e10c15ab79bc14627186))
* rename project to wildgeku ([a0ab15d](https://github.com/ludusrusso/wildgecu/commit/a0ab15da55f192a3fdb83f2ea5043e410b47fe36))
* show toolcalling via TUI ([abe955a](https://github.com/ludusrusso/wildgecu/commit/abe955adeedac25b1fda26698a6ac11c562b4b04))
* support per-session --model override for chat and code commands ([25410bc](https://github.com/ludusrusso/wildgecu/commit/25410bc96f4b8a10b727081110bba47cf3e980ee))
* use Telegram typing indicator instead of "..." placeholder ([#24](https://github.com/ludusrusso/wildgecu/issues/24)) ([b191302](https://github.com/ludusrusso/wildgecu/commit/b191302610eab573d2f874fde703d32816fde959))


### Bug Fixes

* **agent:** prevent nil panic in prompt building and remove stdout noise during tool execution ([4263afa](https://github.com/ludusrusso/wildgecu/commit/4263afacea3e41266dd6f84b0a934f3c1a600e70))
* handle ESC during tool calling ([0b80255](https://github.com/ludusrusso/wildgecu/commit/0b80255943afd912eb4c2a3cbfa3e33aa60dfcc2))
* linting issue ([c434b81](https://github.com/ludusrusso/wildgecu/commit/c434b81559f295ac958504b6550b1a703fdb6da1))
* pass full command string to LLM for skill slash commands ([8f70188](https://github.com/ludusrusso/wildgecu/commit/8f701884de7498f5cf0c3d24e5fb952ca66efd29))
* propagate context to ToolExecutor and harden debug logger ([dccd6d7](https://github.com/ludusrusso/wildgecu/commit/dccd6d77323b0d709df4d2a238cb68493bed4c05))
* resolve all golangci-lint issues ([99aadf9](https://github.com/ludusrusso/wildgecu/commit/99aadf9f9d9d898892211cff85e32f7f2b23fef6))
* resolve lint issues in telegram auth and bridge ([55151ab](https://github.com/ludusrusso/wildgecu/commit/55151abadcd3dba0dc88808e47473bdaa4bac763))
* search ([8f70afc](https://github.com/ludusrusso/wildgecu/commit/8f70afc22b1074bfb72219d4c0d4b553bf4eb6f5))
* update TUI session ID after /clean to prevent stale session errors ([382f265](https://github.com/ludusrusso/wildgecu/commit/382f2655a77f34160572923dd47482747e595941))
* verbs ([3a0df2d](https://github.com/ludusrusso/wildgecu/commit/3a0df2df028b189dd778d484b2b99a93936aceec))
