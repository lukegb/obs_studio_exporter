# obs-studio-exporter

[![Build status](https://ci.appveyor.com/api/projects/status/9q1o274gphwcp4v6?svg=true)](https://ci.appveyor.com/project/lukegb/obs-studio-exporter)

*This is not an official Google product.*

Exports metrics from [OBS Studio](https://obsproject.com) in a [Prometheus](https://prometheus.io)-compatible format.

Listens on port 9407 (currently not configurable).

## Metrics

At present, the following metric groups are exported:

* Global
* Output
* Encoder

### Global

* `obs_global_active_fps`: a *gauge* which contains the current active FPS from OBS.
* `obs_global_average_frame_time_ns`: a *gauge* containing the current average frame time from OBS in nanoseconds.
* `obs_global_total_frames`: a *counter* containing the total frames output by this OBS instance.
* `obs_global_lagged_frames`: a *counter* containing the lagged frames output by this OBS instance.

### Output

* `obs_output_info`: the value is irrelevant, but the labels map the output ID to interesting information about this output.
* `obs_output_active`: a boolean *gauge* indicating if this output is currently active.
* `obs_output_total_bytes`: a *counter* indicating the total bytes output by this output.
* `obs_output_dropped_frames`: a *counter* indicating the total frames dropped by this output.
* `obs_output_total_frames`: a *counter* indicating the total frames sent to this output.
* `obs_output_video_width`: a *gauge* indicating the current output video width.
* `obs_output_video_height`: a *gauge* indicating the current output video height.
* `obs_output_congestion`: a *gauge* estimating the current congestion on this output.
* `obs_output_connect_time_ms`: a *gauge* containing the time taken by this output to connect in milliseconds.
* `obs_output_reconnecting`: a boolean *gauge* indicating if this output is currently reconnecting.

### Encoder

* `obs_encoder_info`: the value is irrelevant, but the labels map the encoder ID to interesting information about this encoder.
* `obs_encoder_active`: a boolean *gauge* indicating if this encoder is currently active.
* `obs_encoder_video_width`: a *gauge* indicating the current output video width.
* `obs_encoder_video_height`: a *gauge* indicating the current output video height.
* `obs_encoder_audio_sample_rate`: a *gauge* indicating the audio sample rate.

## Compiling & Installing

This project is a little bit finnicky to compile and install.

1. `git submodule init && git submodule update`

### Linux

1. Copy `libobs.so` from your OBS 64-bit install (Usually `/usr/lib/libobs.so`) to the root of the exporter checkout directory.
2. `go build -buildmode=c-shared -o obs-studio-exporter.so`
3. Install by copying `obs-studio-exporter.so` to `/usr/lib/obs-plugins/`.

### Windows

1. Copy `obs.dll` from your OBS 64-bit install (from obs-studio/bin/64bit) to the root of the exporter checkout directory.
2. `go build -buildmode=c-shared -o obs-studio-exporter.dll`
3. Install by copying `obs-studio-exporter.dll` to obs-studio/obs-plugins/64bit.

### macOS

1. Copy `libobs.so` from your OBS 64-bit install (Usually `/Applications/OBS.app/Contents/Frameworks/libobs.0.dylib`) to the root of the exporter checkout directory.
2. `go build -buildmode=c-shared -o obs-studio-exporter.so`
3. Install by copying `obs-studio-exporter.so` to `/Applications/OBS.app/Contents/PlugIns/`.
