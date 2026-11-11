// deej OLED: user settings. Everything you might want to change lives in this file.
#pragma once
#include <Arduino.h>
#include "icons.h"

// ---------------------------------------------------------------- display
// 1.3" SH1106 by default. Uncomment for a 0.96" SSD1306 module.
// #define DISPLAY_SSD1306
#define OLED_I2C_ADDRESS 0x3C
#define OLED_BUS_CLOCK   400000UL

// Portrait, for a bar lying left-right with the screen at the right end.
// If the picture is upside down, use U8G2_R1 instead.
#define DISPLAY_ROTATION U8G2_R3

const uint8_t CONTRAST_ACTIVE = 255;
const uint8_t CONTRAST_IDLE   = 16;    // kaomoji screensaver brightness

// ---------------------------------------------------------------- knobs
// In deej slider order (slider 0 first) = left to right on the bar.
// The pots are wired mirrored: A6 is the leftmost knob, A0 the one next to the screen.
// A4/A5 are taken by the OLED (I2C).
const uint8_t NUM_KNOBS = 5;
const uint8_t KNOB_PINS[NUM_KNOBS] = {A6, A3, A2, A1, A0};

// Labels, icons and invert are only the first-boot defaults. The deej app in this repository
// sends the real ones from config.yaml every time it connects, together with which sliders its OS can't drive. They are saved in EEPROM, so the
// screen keeps showing the last ones it got, even with stock deej or no PC.
const uint8_t LABEL_MAX = 10;  // characters; deej cuts labels to this length too
const char *const KNOB_LABELS[NUM_KNOBS] = {"MASTER", "SPOTIFY", "BROWSER", "DISCORD", "ACTIVE"};
const Icon KNOB_ICONS[NUM_KNOBS] = {ICON_SPEAKER, ICON_MUSIC, ICON_GLOBE, ICON_HEADSET, ICON_WINDOW};
// Other icons: ICON_MIC (for 'mic'), ICON_MUTED

// = invert_sliders in deej's config.yaml, so the screen shows the same % as the PC.
// Only the display is affected; deej always gets the raw 0-1023 values.
const bool INVERT_DISPLAY = true;

// Set to false to forget what deej sent and go back to the defaults above on the next boot.
const bool USE_SAVED_LABELS = true;

// The patched deej says "still here" (and which apps are running) every 2 s. After this long
// without it, the screen shows deej as gone: an X in the bottom-left corner and "NO DEEJ".
// Stock deej never sends it, so set SHOW_LINK_STATUS to false if you use stock deej.
const unsigned long HOST_TIMEOUT_MS = 5000;
const bool SHOW_LINK_STATUS = true;

// Signal conditioning (raw ADC counts, 0-1023)
const uint8_t SMOOTHING   = 2;    // moving average weight 1/2^n (higher = smoother, slower)
const uint8_t HYSTERESIS  = 5;    // ~0.5 %: ignore wobble smaller than this
const uint8_t TOUCH_LEVEL = 16;   // ~1.5 %: movement that counts as "knob turned" (wakes the screen)
const uint8_t TOUCH_HOLD  = 3;    // ...and it has to hold for this many samples (rejects noise spikes)
// Per knob, in KNOB_PINS order: the last bit of travel at each end counts as 0 / 100 %, so pots
// that don't quite reach their ends still give a full range. 20 = ~2 %. Knob 3 stops short
// of its end, and that end has been drifting further (raw ~998 -> ~952), so it gets 80 (~8 %)
// until the pot or its 5V joint is fixed. Raise a knob's value if it can't reach 0 or 100 %.
const uint8_t END_ZONE[NUM_KNOBS] = {20, 20, 80, 20, 20};

// ---------------------------------------------------------------- deej serial
const unsigned long SERIAL_BAUD   = 9600;  // = baud_rate in deej's config.yaml
const unsigned long HEARTBEAT_MS  = 1000;  // resend values this often even when nothing moves

// ---------------------------------------------------------------- timing
const unsigned long SPLASH_MS      = 900;
const unsigned long FOCUS_HOLD_MS  = 1200;   // big single-channel view after you stop turning
const unsigned long IDLE_AFTER_MS  = 5000;   // no knob touched -> kaomoji screensaver
const unsigned long OFF_AFTER_MS   = 65000;  // no knob touched -> screen off (0 = never)
const unsigned long KAOMOJI_MS     = 1000;   // screensaver frame time
const unsigned long SAMPLE_MS      = 10;     // knob read interval
const unsigned long FRAME_MS       = 33;     // max ~30 fps redraw

// ---------------------------------------------------------------- screensaver
// Frames shown in order, each at a random spot so no pixel stays lit for long. Plain ASCII only
// (the fonts have no other characters). Up to 4 characters use a big font; longer frames use a
// smaller one that fits about 9 characters across the 64 px wide screen. They're kept in RAM, so
// a few dozen short frames are fine.
const char *const KAOMOJI_FRAMES[] = {
  "(=^_^=)", "(=^-^=)", "(=^w^=)", "(^._.^)", "(=^o^=)", "(^OwO^)", "(^UwU^)", "(=^.^=)",
};
