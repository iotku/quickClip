# QuickClip: Responsive Audio Player with Waveform Visualization

A simple audio player built in Go, utilizing the Beep audio library for audio playback and the responsive Gio UI Library.

[Online DEMO](https://iotku.pw/quickClip/) (Audio files are read *locally* and are **not** sent to a server)
![image](https://github.com/user-attachments/assets/3683c0da-2168-427c-afde-716c1dc75ae7)


## Features

- **Audio Playback**: Supports common audio formats like MP3, WAV, and FLAC.
- **Waveform Visualization**: Displays a real-time waveform of the currently playing audio.
- **Cross-Platform Support**: Runs on Windows, Linux, macOS, and WebAssembly

## Installation

### Dependencies

- [Go](https://go.dev/) Programming Language
- [Beep](https://github.com/gopxl/beep) Audio Playback
- [Gio](https://gioui.org) GUI and Layout
- [dhowden's Tag libaray](https://github.com/dhowden/tag) - Audio Metadata

### Clone the Repository

```sh
git clone https://github.com/iotku/quickClip.git
cd quickClip
```

### Install Dependencies

```sh
go mod tidy
```

### Building and Running on Desktop

```sh
go build -o QuickClip
./QuickClip
```

## Usage

1. Launch the application.
2. Click the "Open" button to launch the file picker and select an audio file to load it into the player.
3. Use the buttons to Play/Stop and seek through the track.
4. When the audio file ends it is removed from playback and you should open a new file.

## License

This project is licensed under the MIT License. See `LICENSE` for details.

## Acknowledgments

- [Beep](https://github.com/gopxl/beep) for audio processing, which uses [Oto](https://github.com/hajimehoshi/oto) as its underlying audio library.
- [Gio](https://gioui.org) for the graphical user interface.
- [tag](https://github.com/dhowden/tag) for reading audio metadata (such as Artist & Title)

