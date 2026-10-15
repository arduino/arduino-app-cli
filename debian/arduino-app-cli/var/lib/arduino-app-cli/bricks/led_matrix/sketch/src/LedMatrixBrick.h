// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: MPL-2.0

#pragma once

#include <ArduinoAppBricks.h>
#include <ArduinoGraphics.h>
#include <Arduino_LED_Matrix.h>
#include <Arduino_RouterBridge.h>
#include <vector>

namespace led_matrix {

namespace {
Arduino_LED_Matrix module;
// The matrix holds no frame of its own, so the brick keeps the last one.
uint8_t frame[8 * 13] = {0};
// Text scrolls on the brick thread, never in the RPC handler: the scroll blocks.
MsgPack::str_t pendingText;
} // namespace

inline void draw(std::vector<uint8_t> newFrame) {
    memcpy(frame, newFrame.data(), min(newFrame.size(), sizeof(frame)));
}

inline void clear() {
    memset(frame, 0, sizeof(frame));
}

inline void print(MsgPack::str_t text) {
    pendingText = text;
}

inline void setup() {
    module.begin();
    module.setGrayscaleBits(3);
    module.textFont(Font_5x7);
    module.textScrollSpeed(100);
    module.clear();
    Bridge.provide("led_matrix/draw", draw);
    Bridge.provide("led_matrix/clear", clear);
    Bridge.provide("led_matrix/print", print);
}

// 4 KiB: the ArduinoGraphics text scroll runs on this stack.
arduino_app_bricks::Loop scroll(
    [] {
        if (pendingText.length() > 0) {
            MsgPack::str_t text = pendingText;
            pendingText = "";

            // The spaces let the text enter from one edge and leave by the other.
            module.beginText(0, 0, 0xFF, 0xFF, 0xFF);
            module.print("      ");
            module.print(text.c_str());
            module.print("      ");
            module.endText(SCROLL_LEFT);
            return;
        }
        module.draw(frame);
    },
    4096);

} // namespace led_matrix
