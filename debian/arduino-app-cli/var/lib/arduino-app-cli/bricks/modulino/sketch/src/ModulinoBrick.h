// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: MPL-2.0

#pragma once

#include <Arduino_Modulino.h>
#include <Arduino_RouterBridge.h>
#include <vector>

namespace modulino {

namespace {
ModulinoButtons buttons;
ModulinoKnob knob;
ModulinoPixels pixels;
ModulinoBuzzer buzzer;
ModulinoDistance distance;
ModulinoThermo thermo;
ModulinoMovement movement;
} // namespace

// One begin per module: the python side calls it for the module it uses, and a false
// answer means the module is not on the bus.

inline bool beginButtons() {
    return buttons.begin();
}

inline bool beginKnob() {
    return knob.begin();
}

inline bool beginPixels() {
    return pixels.begin();
}

inline bool beginBuzzer() {
    return buzzer.begin();
}

inline bool beginDistance() {
    return distance.begin();
}

inline bool beginThermo() {
    return thermo.begin();
}

inline bool beginMovement() {
    return movement.begin();
}

// The three buttons, pressed or not.
inline std::vector<uint8_t> readButtons() {
    buttons.update();
    return {uint8_t(buttons.isPressed(0) == HIGH), uint8_t(buttons.isPressed(1) == HIGH),
            uint8_t(buttons.isPressed(2) == HIGH)};
}

inline void setButtonLeds(bool a, bool b, bool c) {
    buttons.setLeds(a, b, c);
}

// Position, direction of the last turn, and the press of the knob.
inline std::vector<int32_t> readKnob() {
    return {knob.get(), knob.getDirection(), knob.isPressed()};
}

inline void setKnob(int32_t position) {
    knob.set(int16_t(position));
}

inline void setPixel(uint8_t index, uint8_t r, uint8_t g, uint8_t b, uint8_t brightness) {
    pixels.set(index, ModulinoColor(r, g, b), brightness);
}

inline void showPixels() {
    pixels.show();
}

inline void clearPixels() {
    pixels.clear();
    pixels.show();
}

inline void tone(uint32_t frequency, uint32_t lengthMs) {
    buzzer.tone(frequency, lengthMs);
}

inline void noTone() {
    buzzer.noTone();
}

// Distance in mm, or -1 when the module reads nothing.
inline float readDistance() {
    if (!distance.available()) {
        return -1.0f;
    }
    return distance.get();
}

// Temperature in C, humidity in percent.
inline std::vector<float> readThermo() {
    return {thermo.getTemperature(), thermo.getHumidity()};
}

// Acceleration on the three axes, in g.
inline std::vector<float> readMovement() {
    movement.update();
    return {movement.getX(), movement.getY(), movement.getZ()};
}

// Every handler answers at once, so the brick needs no thread of its own.
inline void setup() {
    Modulino.begin(Wire1);

    Bridge.provide("modulino/buttons/begin", beginButtons);
    Bridge.provide("modulino/buttons/read", readButtons);
    Bridge.provide("modulino/buttons/set_leds", setButtonLeds);

    Bridge.provide("modulino/knob/begin", beginKnob);
    Bridge.provide("modulino/knob/read", readKnob);
    Bridge.provide("modulino/knob/set", setKnob);

    Bridge.provide("modulino/pixels/begin", beginPixels);
    Bridge.provide("modulino/pixels/set", setPixel);
    Bridge.provide("modulino/pixels/show", showPixels);
    Bridge.provide("modulino/pixels/clear", clearPixels);

    Bridge.provide("modulino/buzzer/begin", beginBuzzer);
    Bridge.provide("modulino/buzzer/tone", tone);
    Bridge.provide("modulino/buzzer/no_tone", noTone);

    Bridge.provide("modulino/distance/begin", beginDistance);
    Bridge.provide("modulino/distance/read", readDistance);

    Bridge.provide("modulino/thermo/begin", beginThermo);
    Bridge.provide("modulino/thermo/read", readThermo);

    Bridge.provide("modulino/movement/begin", beginMovement);
    Bridge.provide("modulino/movement/read", readMovement);
}

} // namespace modulino
