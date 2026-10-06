# Credits and acknowledgements

| Contributor or project | Contribution or relationship | Rights and reference |
| --- | --- | --- |
| **Mayconrib808** | Bridge project coordination, requirements, testing, distribution and maintenance | Original bridge code and repository documentation: [MIT](LICENSE); [GitHub profile](https://github.com/Mayconrib808) |
| **OpenAI Codex** | Assisted implementation, review, build tooling and repository preparation | Development-tool acknowledgement; no claim of OpenAI sponsorship, certification or support |
| **openOMSI Project and its contributors** | Independently developed simulator; included reference and protocol/ABI adapted by the new Go host | [Official repository](https://github.com/openOMSI-Project/openOMSI); MIT notice credits **usonskyyyy, 2026**, retained [verbatim](source/reference/OPENOMSI_LICENSE.txt) |
| **The Go Authors and credited toolchain contributors** | Go runtime, standard library and compiler used for the bridge programs | [Go source](https://github.com/golang/go); [notices](THIRD_PARTY_NOTICES.md) |
| **PeDePe GbR** | Developer of Bus Company Simulator / Busbetrieb-Simulator, the external compatibility target | [Official website](https://pedepe.de/); their software and rights remain theirs; no proprietary program files are included |
| **MR-Software GbR** | Developer of the original OMSI 2 simulator | [Official product listing](https://store.steampowered.com/app/252530/OMSI_2_Steam_Edition/); no original program or game content is included |
| **Aerosoft GmbH** | Publisher of OMSI 2 | Same [official product listing](https://store.steampowered.com/app/252530/OMSI_2_Steam_Edition/); independent of this bridge |
| **Valve Corporation** | Steam distribution platform used by legitimate installations | [Steam](https://store.steampowered.com/); no Steam files, credentials or service access are supplied |
| **GitHub and the actions/checkout, setup-go and upload-artifact maintainers** | Hosting, CI and downloadable build artifacts | [checkout](https://github.com/actions/checkout), [setup-go](https://github.com/actions/setup-go), [upload-artifact](https://github.com/actions/upload-artifact); actions are referenced, not vendored |
| **Map, bus, repaint and other add-on authors** | Content installed separately by each user | Their own licences continue to apply; no such assets are bundled, so this repository does not claim to credit unknown assets it does not distribute |

Development-only Python and Microsoft Visual Studio/Windows SDK tooling are also acknowledged; their installers/runtime are not redistributed by this ZIP.

These are acknowledgements, not endorsements. No permission from PeDePe, MR-Software, Aerosoft or Valve is asserted. Original notices must accompany copies of the licensed material. Additional verified contributors can be credited through a pull request.

The old custom `omsi-plugin-host32.exe` cannot be assigned a verified author or licence from the available records. It is **not included**. The replacement is built and bundled from attributed MIT source in `source/pluginhost/`; see [the provenance record](docs/HELPER_PROVENANCE.md). The openOMSI MIT notice is not treated as evidence that this particular binary has been cleared.
