# Landing illustration credits

## SunnyLand — Luis Zuno / Ansimuz

- Official source: https://ansimuz.itch.io/sunny-land-pixel-game-art
- Pack used: **Sunny-land-files.zip**, the free pack (not SunnyLand Plus).
- License: **Creative Commons Zero (CC0 1.0)**.
- License terms: https://creativecommons.org/publicdomain/zero/1.0/
- Pack license: `Sunny-land-files/public-license.pdf` states: "All assets included in this package are licensed under the Creative Commons Zero (CC0) license, which means you can use them freely in any project, whether personal or commercial, without the need for attribution. There are no restrictions on use, modification, or redistribution of these assets."

Runtime files and adaptations:

| File | Source inside the free pack | Adaptation |
| --- | --- | --- |
| `clouds.webp` | `Assets/environment/Background/back.png` | Crop to the top 384×113 px; isolate clouds from the cyan sky/ocean, recolor to pale blue, encode lossless WebP. |
| `tree.webp` | `Assets/environment/Props/tree.png` | Lossless WebP conversion, 119×111 px. |
| `bush.webp` | `Assets/environment/Props/bush.png` | Lossless WebP conversion, 46×28 px. |

`HeroScene.svelte` composes these layers with original Singgah skyline, grass, rail, train, laptop, and journal SVG artwork. The scene is decorative illustration, not geography, realtime tracking, or operational data. No Cofounder artwork, video, code, or font files are redistributed.

The component keeps the camera, ground, skyline, laptop chassis, and journal stationary. Trees, bushes, grass blades, clouds, an illustrative train, and the laptop screen have repeating decorative motion. Pause, reduced-motion, document-visibility, and viewport-visibility controls stop every loop. The screen is illustration, not live data.

## Poppins — The Poppins Project Authors

- Official project: https://github.com/itfoundry/Poppins
- License: **SIL Open Font License 1.1**.
- Delivery: installed `@fontsource/poppins` package; Latin normal weights 400, 500, and 600, self-hosted by the Singgah build, without third-party font requests.
- Full original copyright notice and license: [POPPINS-OFL.txt](POPPINS-OFL.txt).
- Scope: hero heading, supporting copy, actions, and the three illustrated steps. Application UI and other landing sections retain Inter.
