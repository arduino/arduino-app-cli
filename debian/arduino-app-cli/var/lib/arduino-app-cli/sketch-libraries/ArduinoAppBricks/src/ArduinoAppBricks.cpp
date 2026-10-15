// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: MPL-2.0

#include "ArduinoAppBricks.h"

#include <Arduino_RouterBridge.h>
#include <zephyr/kernel.h>

namespace arduino_app_bricks {

namespace {
void runBricks(void* startBricks, void*, void*);
void runLoop(void* fn, void* periodMs, void*);

// Set when every brick setup has run.
bool bricksReady = false;

// This image creates threads only before main, so every stack is static.
K_THREAD_STACK_DEFINE(bricksStack, 4096);

constexpr size_t MaxLoops = 4;
constexpr size_t LoopStackSize = 4096;
K_THREAD_STACK_ARRAY_DEFINE(loopStacks, MaxLoops, LoopStackSize);
struct k_thread loopThreads[MaxLoops];
size_t loopsUsed = 0;
} // namespace

Loop::Loop(void (*fn)(), size_t stackSize, int periodMs, int priority) {
    if (loopsUsed == MaxLoops || stackSize > LoopStackSize) {
        return;
    }
    size_t index = loopsUsed++;
    k_thread_create(&loopThreads[index], loopStacks[index], LoopStackSize, runLoop,
                    reinterpret_cast<void*>(fn),
                    reinterpret_cast<void*>(static_cast<intptr_t>(periodMs)), nullptr, priority, 0,
                    K_NO_WAIT);
}

int begin(void (*startBricks)()) {
    // This thread only runs the brick setups. Priority 10, below the sketch.
    static struct k_thread thread;
    k_thread_create(&thread, bricksStack, K_THREAD_STACK_SIZEOF(bricksStack), runBricks,
                    reinterpret_cast<void*>(startBricks), nullptr, nullptr, 10, 0, K_NO_WAIT);
    k_thread_name_set(&thread, "app_bricks");
    return 0;
}

namespace {

void runBricks(void* startBricks, void*, void*) {
    // The app sketch starts the Bridge. A brick must never call Bridge.begin():
    // it wedges a Bridge that already runs.
    while (!Bridge.is_started()) {
        k_msleep(50);
    }
    reinterpret_cast<void (*)()>(startBricks)();
    bricksReady = true;
}

void runLoop(void* fn, void* periodMs, void*) {
    while (!bricksReady) {
        k_msleep(50);
    }
    auto brickLoop = reinterpret_cast<void (*)()>(fn);
    int period = static_cast<int>(reinterpret_cast<intptr_t>(periodMs));
    for (;;) {
        brickLoop();
        // A period of 0 runs the next pass at once.
        if (period == 0) {
            k_yield();
        } else {
            k_msleep(period);
        }
    }
}

} // namespace

} // namespace arduino_app_bricks
