# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

from arduino.app_utils import Bridge, brick


class ModuleNotFound(Exception):
    """The module does not answer on the Qwiic bus."""


class _Module:
    """One Modulino module, reached through the sketch part of the brick."""

    name: str

    def __init__(self) -> None:
        if not Bridge.call(f"modulino/{self.name}/begin"):
            raise ModuleNotFound(f"no {self.name} modulino on the bus")

    def _call(self, method: str, *args):
        return Bridge.call(f"modulino/{self.name}/{method}", *args)


@brick
class ModulinoButtons(_Module):
    """The three buttons and their LEDs."""

    name = "buttons"

    def read(self) -> tuple[bool, bool, bool]:
        """A, B and C, pressed or not."""
        a, b, c = self._call("read")
        return bool(a), bool(b), bool(c)

    def set_leds(self, a: bool, b: bool, c: bool) -> None:
        self._call("set_leds", a, b, c)


@brick
class ModulinoKnob(_Module):
    """The rotary encoder and its push button."""

    name = "knob"

    def read(self) -> tuple[int, int, bool]:
        """Position, direction of the last turn (-1, 0 or 1), and the press."""
        position, direction, pressed = self._call("read")
        return position, direction, bool(pressed)

    def set(self, position: int) -> None:
        self._call("set", position)


@brick
class ModulinoPixels(_Module):
    """The eight RGB LEDs."""

    name = "pixels"

    COUNT = 8
    MAX_BRIGHTNESS = 100

    def set(self, index: int, r: int, g: int, b: int, brightness: int = 25) -> None:
        """Changes one LED. The strip only shows it after show()."""
        if not 0 <= index < self.COUNT:
            raise ValueError(f"the strip has {self.COUNT} LEDs, got index {index}")
        self._call("set", index, r, g, b, brightness)

    def show(self) -> None:
        self._call("show")

    def clear(self) -> None:
        self._call("clear")


@brick
class ModulinoBuzzer(_Module):
    """The buzzer."""

    name = "buzzer"

    def tone(self, frequency: int, length_ms: int) -> None:
        """Plays a tone. Returns at once: the module keeps playing."""
        self._call("tone", frequency, length_ms)

    def no_tone(self) -> None:
        self._call("no_tone")


@brick
class ModulinoDistance(_Module):
    """The time of flight distance sensor."""

    name = "distance"

    def read(self) -> float | None:
        """Distance in mm, or None when the sensor reads nothing."""
        distance = self._call("read")
        return None if distance < 0 else distance


@brick
class ModulinoThermo(_Module):
    """The temperature and humidity sensor."""

    name = "thermo"

    def read(self) -> tuple[float, float]:
        """Temperature in C, and humidity in percent."""
        temperature, humidity = self._call("read")
        return temperature, humidity


@brick
class ModulinoMovement(_Module):
    """The accelerometer."""

    name = "movement"

    def read(self) -> tuple[float, float, float]:
        """Acceleration on x, y and z, in g."""
        x, y, z = self._call("read")
        return x, y, z
