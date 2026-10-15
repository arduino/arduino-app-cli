// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: MPL-2.0

// A brick library declares setup() in a namespace named after the brick.

#pragma once

#include <stddef.h>

namespace arduino_app_bricks {

// A brick thread for blocking work. Declare it at namespace scope, never in setup().
// fn captures nothing, and it starts after every brick setup.
class Loop {
public:
    Loop(void (*fn)(), size_t stackSize, int periodMs = 10, int priority = 10);
};

// Starts a thread that calls startBricks once the Bridge is up. Returns 0.
int begin(void (*startBricks)());

} // namespace arduino_app_bricks
