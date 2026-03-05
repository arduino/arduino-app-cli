# sound_generator API Reference

## Index

- Class `LRUDict`
- Class `SoundGeneratorStreamer`
- Class `SoundGenerator`
- Class `MusicComposition`
- Class `SoundEffect`
- Class `SoundEffectOverdrive`
- Class `SoundEffectChorus`
- Class `SoundEffectADSR`
- Class `SoundEffectTremolo`
- Class `SoundEffectVibrato`
- Class `SoundEffectBitcrusher`
- Class `SoundEffectOctaver`
- Class `WaveSamplesBuilder`
- Class `ABCNotationLoader`

---

## `LRUDict` class

```python
class LRUDict(maxsize)
```

A dictionary-like object with a fixed size that evicts the least recently used items.


---

## `SoundGeneratorStreamer` class

```python
class SoundGeneratorStreamer(bpm: int, time_signature: tuple, octaves: int, wave_form: str, master_volume: float, sound_effects: list)
```

### Parameters

- **bpm** (*int*): The tempo in beats per minute for note duration calculations.
- **time_signature** (*tuple*): The time signature as (numerator, denominator).
- **octaves** (*int*): Number of octaves to generate notes for (starting from octave
0 up to octaves-1).
- **wave_form** (*str*): The type of wave form to generate. Supported values
are "sine" (default), "square", "triangle" and "sawtooth".
- **master_volume** (*float*): The master volume level (0.0 to 1.0).
- **sound_effects** (*list*) (optional): List of sound effect instances to apply to the audio
signal (e.g., [SoundEffect.adsr()]). See SoundEffect class for available effects.

### Methods

#### `set_wave_form(wave_form: str)`

Set the wave form type for sound generation.

##### Parameters

- **wave_form** (*str*): The type of wave form to generate. Supported values
are "sine", "square", "triangle" and "sawtooth".

#### `set_master_volume(volume: float)`

Set the master volume level.

##### Parameters

- **volume** (*float*): Volume level (0.0 to 1.0).

#### `set_bpm(bpm: int)`

Set the tempo in beats per minute.

##### Parameters

- **bpm** (*int*): Tempo in beats per minute.

#### `set_effects(effects: list)`

Set the list of sound effects to apply to the audio signal.

##### Parameters

- **effects** (*list*): List of sound effect instances (e.g., [SoundEffect.adsr()]).

#### `play_polyphonic(notes: list[list[tuple[str, float]]], as_tone: bool, volume: float)`

Generate audio for multiple note sequences mixed together (polyphony).

Produces multi-track audio by mixing a list of sequences, where each
sequence is a list of (note, duration) tuples.

##### Parameters

- **notes** (*list[list[tuple[str, float]]]*): List of sequences, each a list of (note, duration) tuples.
- **as_tone** (*bool*): If True, interpret duration values as seconds instead of note fractions.
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.

##### Returns

- (*tuple[np.ndarray, float]*): The mixed audio block (float32) and its duration in seconds.

#### `play_chord(notes: list[str], note_duration: float | str, volume: float)`

Generate audio for a chord of simultaneous notes.

##### Parameters

- **notes** (*list[str]*): List of musical notes (e.g., ['A4', 'C#5', 'E5']).
- **note_duration** (*float | str*): Duration as a note fraction (like 1/4, 1/8) or symbol ('W', 'H', 'Q', etc.).
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.

##### Returns

- (*np.ndarray*): The audio block of the chord (float32).

#### `play(note: str, note_duration: float | str, volume: float)`

Generate audio samples for a single musical note.

##### Parameters

- **note** (*str*): The musical note to generate (e.g., 'A4', 'C#5', 'REST').
- **note_duration** (*float | str*): Duration as a note fraction (like 1/4, 1/8) or symbol ('W', 'H', 'Q', etc.).
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.

##### Returns

- (*np.ndarray*): The audio block (float32), or None if the note is invalid.

#### `play_tone(note: str, duration: float, volume: float)`

Generate audio samples for a note with duration in seconds.

Unlike ``play()`` which interprets duration as a musical note fraction,
this method takes the duration directly in seconds.

##### Parameters

- **note** (*str*): The musical note to generate (e.g., 'A4', 'C#5', 'REST').
- **duration** (*float*): Duration in seconds (default 0.25).
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.

##### Returns

- (*np.ndarray*): The audio block (float32), or None if the note is invalid.

#### `play_abc(abc_string: str, volume: float)`

Generate audio samples from an ABC notation string.

Yields one audio block per note in the parsed ABC sequence.  The parser
is ABC 2.1 standard compliant (key signatures, accidentals, tuplets,
broken rhythm, multimeasure rests, etc.).  See
:class:`ABCNotationLoader` for the full feature list and limitations.

##### Parameters

- **abc_string** (*str*): ABC notation string defining the sequence of notes.
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.

##### Returns

- (*tuple[np.ndarray, float]*): Audio block (float32) and its duration in seconds.

#### `play_wav(wav_file: str)`

Load a WAV file and return its raw PCM data.

Results are cached (up to 250 KB total) for repeated playback.

##### Parameters

- **wav_file** (*str*): The WAV audio file path.

##### Returns

- (*tuple[bytes, float]*): Raw PCM audio data and its duration in seconds.


---

## `SoundGenerator` class

```python
class SoundGenerator(output_device: Speaker, bpm: int, time_signature: tuple, octaves: int, wave_form: str, master_volume: float, sound_effects: list)
```

### Parameters

- **output_device** (*Speaker*) (optional): The output device to play sound through.
- **bpm** (*int*): The tempo in beats per minute for note duration calculations.
- **time_signature** (*tuple*): The time signature as (numerator, denominator).
- **octaves** (*int*): Number of octaves to generate notes for (starting from octave
0 up to octaves-1).
- **wave_form** (*str*): The type of wave form to generate. Supported values
are "sine" (default), "square", "triangle" and "sawtooth".
- **master_volume** (*float*): The master volume level (0.0 to 1.0).
- **sound_effects** (*list*) (optional): List of sound effect instances to apply to the audio
signal (e.g., [SoundEffect.adsr()]). See SoundEffect class for available effects.

### Methods

#### `start()`

Start the sound generator and its internal speaker (if not external).

#### `stop()`

Stop playback, halt any running sequence, and close the internal speaker.

#### `set_master_volume(volume: float)`

Set the master volume level.

##### Parameters

- **volume** (*float*): Volume level (0.0 to 1.0).

#### `set_effects(effects: list)`

Set the list of sound effects to apply to the audio signal.

##### Parameters

- **effects** (*list*): List of sound effect instances (e.g., [SoundEffect.adsr()]).

#### `play_polyphonic(notes: list[list[tuple[str, float]]], as_tone: bool, volume: float, block: bool)`

Play multiple sequences of musical notes simultaneously (poliphony).

It is possible to play multi track music by providing a list of sequences,
where each sequence is a list of tuples (note, duration).
Duration is in notes fractions (e.g., 1/4 for quarter note).

##### Parameters

- **notes** (*list[list[tuple[str, float]]]*): List of sequences, each sequence is a list of tuples (note, duration).
- **as_tone** (*bool*): If True, play as tones, considering duration in seconds
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.
- **block** (*bool*): If True, block until the entire sequence has been played.

#### `play_composition(composition: 'MusicComposition', block: bool)`

Play a MusicComposition object.

Configures the SoundGenerator with the composition's settings and plays
the sequence using play_step_sequence.

The composition format is interpreted as a list of steps, where each step
is a list of (note, duration) tuples to play simultaneously.

##### Parameters

- **composition** (*MusicComposition*): The composition to play.
- **block** (*bool*): If True, block until the entire composition has been played.

#### `play_chord(notes: list[str], note_duration: float | str, volume: float, block: bool)`

Play a chord consisting of multiple musical notes simultaneously for a specified duration and volume.

##### Parameters

- **notes** (*list[str]*): List of musical notes to play (e.g., ['A4', 'C#5', 'E5']).
- **note_duration** (*float | str*): Duration of the chord as a float (like 1/4, 1/8) or a symbol ('W', 'H', 'Q', etc.).
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.
- **block** (*bool*): If True, block until the entire chord has been played.

#### `play(note: str, note_duration: float | str, volume: float, block: bool)`

Play a musical note for a specified duration and volume.

##### Parameters

- **note** (*str*): The musical note to play (e.g., 'A4', 'C#5', 'REST').
- **note_duration** (*float | str*): Duration of the note as a float (like 1/4, 1/8) or a symbol ('W', 'H', 'Q', etc.).
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.
- **block** (*bool*): If True, block until the entire note has been played.

#### `play_tone(note: str, duration: float, volume: float, block: bool)`

Play a musical note with duration specified in seconds.

Unlike ``play()`` which interprets duration as a musical note fraction,
this method takes the duration directly in seconds.

##### Parameters

- **note** (*str*): The musical note to play (e.g., 'A4', 'C#5', 'REST').
- **duration** (*float*): Duration in seconds (default 0.25).
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.
- **block** (*bool*): If True, block until the entire note has been played.

#### `play_abc(abc_string: str, volume: float, block: bool)`

Play a sequence of musical notes defined in ABC notation.

The parser is ABC 2.1 standard compliant (key signatures, accidentals,
tuplets, broken rhythm, multimeasure rests, etc.).  See
:class:`ABCNotationLoader` for the full feature list and limitations.

##### Parameters

- **abc_string** (*str*): ABC notation string defining the sequence of notes.
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.
- **block** (*bool*): If True, block until the entire sequence has been played.

#### `play_wav(wav_file: str, block: bool)`

Play a WAV audio file through the output device.

##### Parameters

- **wav_file** (*str*): The WAV audio file path.
- **block** (*bool*): If True, block until the entire WAV file has been played.

#### `play_step_sequence(sequence: list[list[str]], note_duration: float | str, bpm: int, loop: bool, on_step_callback: callable, on_complete_callback: callable, volume: float)`

Play a step sequence with automatic timing.

This method handles all the complexity of buffer management internally,
allowing the app to simply provide the sequence and let the brick manage playback.

##### Parameters

- **sequence** (*list[list[str]]*): List of steps, where each step is a list of notes.
Empty list or None means REST (silence) for that step.
Example: [['C4'], ['E4', 'G4'], [], ['C5']]
- **note_duration** (*float | str*): Duration of each step as a float (like 1/16) or symbol ('E', 'Q', etc.).
- **bpm** (*int*) (optional): Tempo in beats per minute. If None, uses instance BPM.
- **loop** (*bool*): If True, the sequence will loop indefinitely until stop_sequence() is called.
- **on_step_callback** (*callable*) (optional): Callback function called for each step.
Signature: on_step_callback(current_step: int, total_steps: int)
- **on_complete_callback** (*callable*) (optional): Callback function called when sequence completes (only if loop=False).
Signature: on_complete_callback()
- **volume** (*float*) (optional): Volume level (0.0 to 1.0). If None, uses master volume.

##### Returns

- (*None*): Returns immediately after starting playback thread.

##### Examples

```python
```python
# Simple melody with chords
sequence = [
    ["C4"],  # Step 0: Single note
    ["E4", "G4"],  # Step 1: Chord
    [],  # Step 2: REST
    ["C5"],  # Step 3: High note
]
sound_gen.play_step_sequence(sequence, note_duration=1 / 16, bpm=120)
```
```
#### `stop_sequence()`

Stop the currently playing step sequence.

Signals the playback thread to stop and closes the internal speaker to
immediately drop any pending audio in the ALSA buffer.  The speaker is
transparently restarted on the next play call via _ensure_speaker_ready.

#### `is_sequence_playing()`

Check if a step sequence is currently playing.

##### Returns

- (*bool*): True if a sequence is playing, False otherwise.


---

## `MusicComposition` class

```python
class MusicComposition(composition: list[list[tuple[str, float]]], bpm: int, waveform: str, volume: float, effects: list)
```

A structured representation of a musical composition for SoundGenerator.

This class encapsulates all the parameters needed to play a polifonic step sequence,
making it easy to save, load, and share musical sequences.

### Attributes

- **composition** (*list[list[tuple[str, float]]]*): Polyphonic sequence as a list of tracks.
Each track is a list of tuples (note, duration).
Duration is in note fractions (1/4 = quarter note, 1/8 = eighth note).
Example: [[("C4", 0.25), ("E4", 0.25)], [("G4", 0.5)]]
- **bpm** (*int*): Tempo in beats per minute. Default: 120.
- **waveform** (*str*): Wave form type ("sine", "square", "triangle", "sawtooth"). Default: "sine".
- **volume** (*float*): Master volume level (0.0 to 1.0). Default: 0.8.
- **effects** (*list*): List of SoundEffect instances to apply. Default: [SoundEffect.adsr()].


---

## `SoundEffect` class

```python
class SoundEffect()
```

### Methods

#### `overdrive(drive: float)`

Apply overdrive effect to the audio signal.

##### Parameters

- **signal** (*np.ndarray*): Input audio signal.
- **drive** (*float*): Overdrive intensity factor.

##### Returns

- (*np.ndarray*): Processed audio signal with overdrive effect.

#### `chorus(depth_ms, rate_hz: float, mix: float)`

Apply chorus effect to the audio signal.

##### Parameters

- **signal** (*np.ndarray*): Input audio signal.
- **depth_ms** (*float*): Depth of the chorus effect in milliseconds.
- **rate_hz** (*float*): Rate of the LFO in Hz.
- **mix** (*float*): Mix ratio between dry and wet signals (0.0 to 1.0).

##### Returns

- (*np.ndarray*): Processed audio signal with chorus effect.

#### `adsr(attack: float, decay: float, sustain: float, release: float)`

Apply ADSR (attack/decay/sustain/release) envelope to the audio signal.

##### Parameters

- **attack** (*float*): Attack time in seconds.
- **decay** (*float*): Decay time in seconds.
- **sustain** (*float*): Sustain level (0.0 to 1.0).
- **release** (*float*): Release time in seconds.


---

## `SoundEffectOverdrive` class

```python
class SoundEffectOverdrive(drive: float)
```


---

## `SoundEffectChorus` class

```python
class SoundEffectChorus(depth_ms: int, rate_hz: float, mix: float)
```


---

## `SoundEffectADSR` class

```python
class SoundEffectADSR(attack: float, decay: float, sustain: float, release: float)
```

### Parameters

- **attack** (*float*): Attack time in seconds.
- **decay** (*float*): Decay time in seconds.
- **sustain** (*float*): Sustain level (0.0 to 1.0).
- **release** (*float*): Release time in seconds.

### Methods

#### `apply(signal: np.ndarray)`

Apply ADSR filter on signal.

##### Parameters

- **signal**: np.ndarray float32 (audio)


---

## `SoundEffectTremolo` class

```python
class SoundEffectTremolo(depth: float, rate: float)
```

### Parameters

- **depth** (*float*): modulation depth (0=no effect, 1=full)
- **rate** (*float*): rate in cycles per block

### Methods

#### `apply(signal: np.ndarray)`

Apply tremolo to a block of audio.

##### Parameters

- **signal** (*np.ndarray*): input block


---

## `SoundEffectVibrato` class

```python
class SoundEffectVibrato(depth: float, rate: float)
```

### Parameters

- **depth** (*float*): max deviation (0=no effect, 0.5=max)
- **rate** (*float*): number of cycles per block


---

## `SoundEffectBitcrusher` class

```python
class SoundEffectBitcrusher(bits: int, reduction: int)
```

### Parameters

- **bits** (*int*): Bit depth for quantization (1-16).
- **reduction** (*int*): Redeuction factor for downsampling (>=1).


---

## `SoundEffectOctaver` class

```python
class SoundEffectOctaver(oct_up: bool, oct_down: bool)
```

### Parameters

- **oct_up** (*bool*): Add one octave above the original signal.
- **oct_down** (*bool*): Add one octave below the original signal.

### Methods

#### `apply(signal: np.ndarray)`

Apply the octaver effect to a mono audio signal.

signal: numpy array with float values in range [-1, 1]


---

## `WaveSamplesBuilder` class

```python
class WaveSamplesBuilder(wave_form: str, sample_rate: int)
```

Generate wave audio blocks.

This class produces wave blocks as NumPy buffers.

### Parameters

- **wave_form** (*str*): The type of wave form to generate. Supported values
are "sine", "square", "triangle", "white_noise" and "sawtooth".
- **sample_rate** (*int*): The playback sample rate (Hz) used to compute
phase increments and buffer sizes.

### Attributes

- **sample_rate** (*int*): Audio sample rate in Hz.

### Methods

#### `generate_block(freq: float, block_dur: float, master_volume: float)`

Generate a block of float32 audio samples.

Returned buffer is a NumPy view (float32) into an internal preallocated array and is valid
until the next call to this method.

##### Parameters

- **freq** (*float*): Target frequency in Hz for this block.
- **block_dur** (*float*): Duration of the requested block in seconds.
- **master_volume** (*float*) (optional): Global gain multiplier. Defaults
to 1.0.

##### Returns

- (*numpy.ndarray*): A 1-D float32 NumPy array containing the generated
audio samples for the requested block.


---

## `ABCNotationLoader` class

```python
class ABCNotationLoader()
```

ABC notation parser — ABC 2.1 standard compliant.

Parses ABC notation strings into ``(note, duration_in_seconds)`` tuples
suitable for playback through the SoundGenerator brick.

Supported ABC 2.1 features:
    - Information fields: ``X:``, ``T:``, ``M:``, ``L:``, ``Q:``, ``K:``
    - Key signatures: all major/minor keys, modes (dorian … locrian),
      ``K:none``, ``K:Hp``/``HP``, ``exp`` (explicit), inline accidental
      overrides (e.g. ``K:D =f ^c``)
    - Accidentals: prefix ``^``, ``^^``, ``_``, ``__``, ``=`` with
      bar-local propagation (pitch-class scope)
    - Octave modifiers: ``'`` (up) and ``,`` (down), case-based octave
    - Duration notation: integer multiplier, ``/n``, ``n/m``, repeated
      slashes (``//`` = ``/4``, ``///`` = ``/8``)
    - Rests: ``z`` (visible), ``x`` (invisible), ``Z``/``X``
      (multimeasure, duration computed from ``M:``)
    - Broken rhythm: ``>``, ``>>``, ``<``, ``<<``
    - Tuplets: ``(p``, ``(p:q``, ``(p:q:r``
    - Chord brackets: ``[CEG]`` (flattened to sequential notes)
    - Grace notes ``{abc}``, decorations ``!ff!`` / ``+fermata+``,
      chord annotations ``"Cm"`` — stripped during pre-processing
    - Non-standard extension: ``%%transpose`` (octave shift)
    - Legacy extension: suffix ``#`` / ``b`` accidentals

Known limitations (not implemented):
    - Multi-voice scores (``V:`` fields)
    - Repeat structures (``|:`` … ``:|``, numbered endings)
    - Ties (``-``) and slurs (``()``)
    - Inline information fields (``[K:Am]``)
    - ``%%propagate-accidentals`` directive (fixed pitch-class scope)
    - ``K:`` clef / transpose parameters
    - ``w:`` lyrics, ``s:`` symbol lines

### Methods

#### `parse_abc_notation(abc_string: str, default_octave: int)`

Parse an ABC notation string into ``(note, duration_in_seconds)`` tuples.

See :class:`ABCNotationLoader` for the full list of supported ABC 2.1
features and known limitations.

##### Parameters

- **abc_string** (*str*): ABC notation string.
- **default_octave** (*int*): Default octave for uppercase notes (C4).

##### Returns

- (*Tuple[dict, List[Tuple[str, float]]]*): Metadata dictionary and list
of (note, duration) tuples.

