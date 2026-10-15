# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

from collections.abc import Sequence

from arduino.app_utils import Bridge, brick

ROWS = 8
COLS = 13

MAX_BRIGHTNESS = 7


@brick
class LedMatrix:
    """Draws on the board LED matrix, through the sketch part of the brick."""

    def draw(self, frame: Sequence[int]) -> None:
        """Draws ROWS * COLS brightness values, 0 to MAX_BRIGHTNESS, row by row."""
        if len(frame) != ROWS * COLS:
            raise ValueError(f"a frame holds {ROWS * COLS} values, got {len(frame)}")
        Bridge.call("led_matrix/draw", bytes(frame))

    def print(self, text: str) -> None:
        """Scrolls text once, left. Replaces what the matrix shows."""
        Bridge.call("led_matrix/print", text)

    def clear(self) -> None:
        Bridge.call("led_matrix/clear")
