// deej-5-sliders-oled: 5 knobs and a 1.3" SH1106 (or 0.96" SSD1306) I2C OLED on an Arduino Nano.
//
// Talks the standard deej protocol ("v0|v1|v2|v3|v4\r\n", raw 0-1023 per knob), so stock deej
// still works. The deej in this repository also sends each slider's label, icon, whether the PC's
// OS can drive it and whether its app is running, so the screen follows config.yaml on Windows
// and Linux without reflashing (see pkg/deej/display.go, and the lines described above
// handleHostLine()). Whatever deej sent last is kept in EEPROM. On the screen (portrait, bars
// lined up with the knobs):
//   overview  one bar per knob with its icon and level; dotted bar + "NA" = not on this OS
//   focus     the knob you are turning, shown big, for a moment after you let go
//   idle      kaomoji screensaver, dimmed and drifting (OLED burn-in), then the screen turns off
//
// Needs the U8g2 library (Arduino IDE: Tools > Manage Libraries > "U8g2").
// All settings are in config.h.

#include <Arduino.h>
#include <Wire.h>
#include <avr/eeprom.h>
#include <U8g2lib.h>
#include "config.h"

#ifdef DISPLAY_SSD1306
U8G2_SSD1306_128X64_NONAME_2_HW_I2C oled(DISPLAY_ROTATION, U8X8_PIN_NONE);
#define DISPLAY_NAME "0.96\" SSD1306"
#else
U8G2_SH1106_128X64_NONAME_2_HW_I2C oled(DISPLAY_ROTATION, U8X8_PIN_NONE);
#define DISPLAY_NAME "1.3\" SH1106"
#endif

const uint8_t NUM_KAOMOJI = sizeof(KAOMOJI_FRAMES) / sizeof(KAOMOJI_FRAMES[0]);
const uint8_t LINE_MAX = NUM_KNOBS * 5 + 1;  // "1023|" per knob, last one "1023\r\n"

// Portrait layout, 64 x 128
const uint8_t COL_X0 = 2;
const uint8_t COL_W = 12;
const uint8_t BAR_Y = 13;
const uint8_t BAR_H = 92;

enum Mode : uint8_t { MODE_ACTIVE, MODE_IDLE, MODE_OFF };
enum View : uint8_t { VIEW_OVERVIEW, VIEW_FOCUS, VIEW_KAOMOJI, VIEW_NONE };

// ---------------------------------------------------------------- state
uint16_t filtered[NUM_KNOBS];  // moving average, fixed point (x16)
uint16_t value[NUM_KNOBS];     // settled 0-1023 value: what deej gets and the screen shows
uint16_t touchRef[NUM_KNOBS];  // last position that counted as a touch
uint8_t touchCount[NUM_KNOBS]; // consecutive samples away from touchRef

Mode mode = MODE_ACTIVE;
View shownView = VIEW_NONE;
int8_t focusKnob = -1;
bool valuesChanged = true;
bool dirty = true;
unsigned long lastTouchMs, focusMs, lastSampleMs, lastSendMs, lastDrawMs, lastFrameMs;

uint8_t kaomojiFrame = 0;
int16_t kaomojiX, kaomojiY;

// ---------------------------------------------------------------- labels
// What each knob controls: config.h defaults, then the EEPROM copy, then whatever deej sends.
const uint8_t FLAG_OK = 0, FLAG_UNSUPPORTED = 1, FLAG_UNMAPPED = 2;  // same numbers deej sends
const uint8_t OS_MAX = 7;  // "WINDOWS"

struct Labels {
  uint8_t magic;  // LABELS_MAGIC when the EEPROM copy is valid. Change it if this struct changes.
  uint8_t numKnobs;
  bool invert;
  char os[OS_MAX + 1];  // uppercase, "" until deej has said
  uint8_t icon[NUM_KNOBS];
  uint8_t flags[NUM_KNOBS];
  char label[NUM_KNOBS][LABEL_MAX + 1];
};
const uint8_t LABELS_MAGIC = 0xD1;
Labels EEMEM savedInfo;  // the EEPROM copy
Labels info;

// icon names deej sends, in Icon enum order (see icons.h / tools/gen_icons.go)
const char ICON_NAME_0[] PROGMEM = "speaker";
const char ICON_NAME_1[] PROGMEM = "music";
const char ICON_NAME_2[] PROGMEM = "globe";
const char ICON_NAME_3[] PROGMEM = "headset";
const char ICON_NAME_4[] PROGMEM = "window";
const char ICON_NAME_5[] PROGMEM = "mic";
const char ICON_NAME_6[] PROGMEM = "muted";
const char ICON_NAME_7[] PROGMEM = "game";
const char ICON_NAME_8[] PROGMEM = "chat";
const char ICON_NAME_9[] PROGMEM = "video";
const char ICON_NAME_10[] PROGMEM = "micoff";
const char *const ICON_NAMES[] PROGMEM = {ICON_NAME_0, ICON_NAME_1, ICON_NAME_2, ICON_NAME_3,
                                          ICON_NAME_4, ICON_NAME_5, ICON_NAME_6, ICON_NAME_7,
                                          ICON_NAME_8, ICON_NAME_9, ICON_NAME_10};
const uint8_t NUM_ICON_NAMES = sizeof(ICON_NAMES) / sizeof(ICON_NAMES[0]);
static_assert(NUM_ICON_NAMES == ICON_COUNT, "ICON_NAMES must list every icon in icons.h, in order");

// Live state from deej's ">A" keepalive
uint8_t activeMask = 0xFF;     // bit per knob: its app is running (all "running" until told)
bool hostSeen = false;         // got any line from the patched deej since power-up
unsigned long lastHostMs;      // when, stamped with the loop's `now`
bool linkShown = false;        // the link state on screen, to redraw when it changes

const uint8_t HOST_LINE_MAX = 31;  // ">S 4 headset 1 " + a 10-character label fits
char hostLine[HOST_LINE_MAX + 1];
uint8_t hostLen = 0;
bool hostOverflow = false;
bool hostGotSlider = false;

void setDefaultLabels() {
  memset(&info, 0, sizeof(info));
  info.numKnobs = NUM_KNOBS;
  info.invert = INVERT_DISPLAY;
  for (uint8_t i = 0; i < NUM_KNOBS; i++) {
    info.icon[i] = KNOB_ICONS[i];
    info.flags[i] = FLAG_OK;
    strncpy(info.label[i], KNOB_LABELS[i], LABEL_MAX);
  }
}

void loadLabels() {
  if (USE_SAVED_LABELS) {
    eeprom_read_block(&info, &savedInfo, sizeof(info));
    if (info.magic == LABELS_MAGIC && info.numKnobs == NUM_KNOBS) {
      info.os[OS_MAX] = 0;  // never trust the EEPROM blindly
      for (uint8_t i = 0; i < NUM_KNOBS; i++) {
        if (info.icon[i] >= ICON_COUNT) info.icon[i] = ICON_SPEAKER;
        if (info.flags[i] > FLAG_UNMAPPED) info.flags[i] = FLAG_UNSUPPORTED;
        info.label[i][LABEL_MAX] = 0;
      }
      return;
    }
  }
  setDefaultLabels();
}

uint8_t iconByName(const char *name, uint8_t fallback) {
  for (uint8_t k = 0; k < NUM_ICON_NAMES; k++) {
    if (strcmp_P(name, (PGM_P)pgm_read_word(&ICON_NAMES[k])) == 0) return k;
  }
  return fallback;
}

// copies printable ASCII, at most `max` characters, without trailing spaces
void copyText(char *dst, const char *src, uint8_t max, bool upper) {
  uint8_t n = 0;
  for (; *src && n < max; src++) {
    char c = *src;
    if (c < 0x20 || c > 0x7e) continue;
    dst[n++] = upper ? toupper(c) : c;
  }
  while (n && dst[n - 1] == ' ') n--;
  dst[n] = 0;
}

const char *skipSpaces(const char *p) {
  while (*p == ' ') p++;
  return p;
}

// Lines from deej, each ending in "\n" (see pkg/deej/display.go):
//   >H <sliders> <invert 0|1> <os>     header, e.g. ">H 5 1 linux"
//   >S <index> <icon> <flags> <label>  one per slider, e.g. ">S 4 window 1 ACTIVE"
//   >E                                 end: save to EEPROM
//   >A <mask>                          every 2 s: bit i set = knob i's app is running
// flags: 0 = works, 1 = not supported on this OS, 2 = not mapped in config.yaml.
// Anything else (and any slider index we don't have) is ignored.
// `now` is the loop's timestamp (see sampleKnobs for why it must not be a fresh millis()).
void handleHostLine(char *s, unsigned long now) {
  if (s[0] != '>') return;
  char *p = s + 2;

  switch (s[1]) {
    case 'A': {
      uint8_t mask = strtoul(p, &p, 10);
      hostSeen = true;
      lastHostMs = now;
      if (mask != activeMask) {
        activeMask = mask;
        dirty = true;
      }
      return;  // every 2 s: only redraw when something changed
    }
    case 'H': {
      strtoul(p, &p, 10);  // slider count: we only use the ones we have
      info.invert = strtoul(p, &p, 10) != 0;
      copyText(info.os, skipSpaces(p), OS_MAX, true);
      hostGotSlider = false;
      break;
    }
    case 'S': {
      unsigned long i = strtoul(p, &p, 10);
      if (i >= NUM_KNOBS) return;
      p = (char *)skipSpaces(p);
      char *name = p;
      while (*p && *p != ' ') p++;
      if (*p) *p++ = 0;
      unsigned long f = strtoul(p, &p, 10);
      if (*p == ' ') p++;

      info.icon[i] = iconByName(name, info.icon[i]);  // unknown name: keep the current icon
      info.flags[i] = f <= FLAG_UNMAPPED ? f : FLAG_UNSUPPORTED;
      copyText(info.label[i], p, LABEL_MAX, false);
      if (!info.label[i][0]) strcpy(info.label[i], "?");
      hostGotSlider = true;
      break;
    }
    case 'E':
      if (hostGotSlider) {
        info.magic = LABELS_MAGIC;
        info.numKnobs = NUM_KNOBS;
        if (USE_SAVED_LABELS) eeprom_update_block(&info, &savedInfo, sizeof(info));  // changed bytes only
      }
      hostGotSlider = false;
      break;
    default:
      return;
  }
  hostSeen = true;
  lastHostMs = now;
  dirty = true;
}

// Never blocks: takes whatever is in the RX buffer and handles complete lines.
void readHost(unsigned long now) {
  while (Serial.available()) {
    char c = Serial.read();
    if (c == '\n' || c == '\r') {
      if (hostLen && !hostOverflow) {
        hostLine[hostLen] = 0;
        handleHostLine(hostLine, now);
      }
      hostLen = 0;
      hostOverflow = false;
    } else if (hostLen < HOST_LINE_MAX) {
      hostLine[hostLen++] = c;
    } else {
      hostOverflow = true;  // too long to be ours: drop it
    }
  }
}

// deej is running and talking to us (the patched one sends ">A" every 2 s)
bool linkUp(unsigned long now) {
  return hostSeen && now - lastHostMs < HOST_TIMEOUT_MS;
}

// the knob's app has audio right now. Without deej we don't know, so assume it does.
bool appRunning(uint8_t i, unsigned long now) {
  return !linkUp(now) || (activeMask >> i) & 1;
}

// ---------------------------------------------------------------- knobs
uint16_t readKnob(uint8_t pin) {
  analogRead(pin);  // throw away the first read after switching channels
  uint16_t sum = 0;
  for (uint8_t i = 0; i < 4; i++) sum += analogRead(pin);
  return sum >> 2;
}

uint16_t absDiff(uint16_t a, uint16_t b) { return a > b ? a - b : b - a; }

// Returns true when a knob was actually turned (not just noise). `now` must be the same
// timestamp the caller uses for its view decisions: a later millis() here would make
// `now - focusMs` wrap around and drop the focus view for a frame.
bool sampleKnobs(unsigned long now) {
  bool touched = false;
  for (uint8_t i = 0; i < NUM_KNOBS; i++) {
    int16_t target = readKnob(KNOB_PINS[i]) << 4;
    filtered[i] += (target - (int16_t)filtered[i]) >> SMOOTHING;

    // stretch [END_ZONE, 1023 - END_ZONE] to the full 0-1023 range
    const uint8_t ez = END_ZONE[i];
    uint16_t v = (filtered[i] + 8) >> 4;
    if (v <= ez) v = 0;
    else if (v >= 1023 - ez) v = 1023;
    else v = (uint32_t)(v - ez) * 1023 / (1023 - 2 * ez);

    bool atEnd = (v == 0 || v == 1023) && v != value[i];
    if (atEnd || absDiff(v, value[i]) >= HYSTERESIS) {
      value[i] = v;
      valuesChanged = true;
      if (mode == MODE_ACTIVE) {
        dirty = true;
        if (i == focusKnob) focusMs = now;  // fine-tuning keeps the focus view up
      }
    }
    if (absDiff(v, touchRef[i]) < TOUCH_LEVEL) {
      touchCount[i] = 0;
    } else if (++touchCount[i] >= TOUCH_HOLD) {
      touchCount[i] = 0;
      touchRef[i] = v;
      focusKnob = i;
      touched = true;
    }
  }
  return touched;
}

// Same rounding as deej (floor to whole percent), so the screen matches the PC.
uint8_t percent(uint8_t i) {
  uint8_t p = (uint32_t)value[i] * 100 / 1023;
  return info.invert ? 100 - p : p;
}

// ---------------------------------------------------------------- deej serial
void sendValues() {
  char line[LINE_MAX + 1];
  uint8_t n = 0;
  for (uint8_t i = 0; i < NUM_KNOBS; i++) {
    if (i) line[n++] = '|';
    utoa(value[i], line + n, 10);
    n += strlen(line + n);
  }
  line[n++] = '\r';
  line[n++] = '\n';
  Serial.write(line, n);
}

// ---------------------------------------------------------------- drawing helpers
void drawCentered(const char *s, int16_t baseline) {
  oled.drawStr((oled.getDisplayWidth() - oled.getStrWidth(s)) / 2, baseline, s);
}

void drawPercent(uint8_t p, int16_t baseline) {  // big number with a small % sign, centred
  char buf[4];
  utoa(p, buf, 10);
  oled.setFont(u8g2_font_6x10_tr);
  uint8_t pw = oled.getStrWidth("%");
  oled.setFont(u8g2_font_logisoso28_tn);
  uint8_t w = oled.getStrWidth(buf);
  int16_t x = (oled.getDisplayWidth() - w - pw - 1) / 2;
  oled.drawStr(x, baseline, buf);
  oled.setFont(u8g2_font_6x10_tr);
  oled.drawStr(x + w + 1, baseline, "%");
}

void drawDottedFrame(int16_t x, int16_t y, int16_t w, int16_t h) {  // "this knob does nothing"
  for (int16_t i = 1; i < w - 1; i += 2) {
    oled.drawPixel(x + i, y);
    oled.drawPixel(x + i, y + h - 1);
  }
  for (int16_t j = 1; j < h - 1; j += 2) {
    oled.drawPixel(x, y + j);
    oled.drawPixel(x + w - 1, y + j);
  }
}

void drawHatchedBox(int16_t x, int16_t y, int16_t w, int16_t h) {  // a "dimmed" fill
  for (int16_t j = 0; j < h; j++) {
    for (int16_t i = (j & 1); i < w; i += 2) oled.drawPixel(x + i, y + j);
  }
}

void drawLinkDown() {  // small X in the footer's left corner: deej isn't talking to us
  oled.drawLine(0, 122, 4, 126);
  oled.drawLine(0, 126, 4, 122);
}

// the icon for a knob at 0 %: a crossed-out mic for mic knobs, a muted speaker otherwise
uint8_t mutedIcon(uint8_t i) {
  return info.icon[i] == ICON_MIC ? ICON_MICOFF : ICON_MUTED;
}

// ---------------------------------------------------------------- screens
void drawSplash() {
  oled.setFont(u8g2_font_logisoso20_tr);
  drawCentered("deej", 58);
  oled.drawHLine(14, 66, 36);
  oled.setFont(u8g2_font_5x7_tr);
  drawCentered("VOLUME MIXER", 80);
  drawCentered(DISPLAY_NAME, 92);
  if (info.os[0]) drawCentered(info.os, 108);  // the OS deej last told us about
}

void drawOverview(unsigned long now) {
  bool link = linkUp(now);
  for (uint8_t i = 0; i < NUM_KNOBS; i++) {
    uint8_t x = COL_X0 + i * COL_W;
    uint8_t p = percent(i);
    const uint8_t *icon = ICONS_8[info.icon[i]];

    // icon, inverted on the last knob you touched
    if (i == focusKnob) {
      oled.drawRBox(x, 0, COL_W, 10, 2);
      oled.setDrawColor(0);
      oled.drawXBMP(x + 2, 1, 8, 8, icon);
      oled.setDrawColor(1);
    } else {
      oled.drawXBMP(x + 2, 1, 8, 8, icon);
    }

    // not on this OS / not mapped: dotted empty bar and "NA" / "--" instead of a level
    if (info.flags[i] != FLAG_OK) {
      drawDottedFrame(x + 2, BAR_Y, 8, BAR_H);
      const char *s = info.flags[i] == FLAG_UNSUPPORTED ? "NA" : "--";
      oled.setFont(u8g2_font_tom_thumb_4x6_tr);
      oled.drawStr(x + (COL_W - oled.getStrWidth(s)) / 2, 114, s);
      continue;
    }

    // bar, filling from the bottom with a 1 px gap inside the frame; hatched when the app
    // isn't running, so you can see what the knob is set to but that nothing is playing
    oled.drawRFrame(x + 2, BAR_Y, 8, BAR_H, 2);
    uint8_t h = (uint16_t)(BAR_H - 4) * p / 100;
    if (h && appRunning(i, now)) oled.drawBox(x + 4, BAR_Y + BAR_H - 2 - h, 4, h);
    else if (h) drawHatchedBox(x + 4, BAR_Y + BAR_H - 2 - h, 4, h);

    // level
    if (p == 0) {
      oled.drawXBMP(x + 2, 108, 8, 8, ICONS_8[mutedIcon(i)]);
    } else {
      char buf[4];
      utoa(p, buf, 10);
      oled.setFont(u8g2_font_tom_thumb_4x6_tn);
      oled.drawStr(x + (COL_W - oled.getStrWidth(buf)) / 2, 114, buf);
    }
  }

  // footer: "NO DEEJ" while deej is gone, else the name of the last knob you touched, else the
  // PC's OS. (focusKnob stays set after the first touch, so "NO DEEJ" has to win over it.)
  oled.drawHLine(0, 119, oled.getDisplayWidth());
  oled.setFont(u8g2_font_5x7_tr);
  const char *footer = focusKnob >= 0 ? info.label[focusKnob] : (info.os[0] ? info.os : "deej");
  if (SHOW_LINK_STATUS && !link) {
    drawLinkDown();
    footer = "NO DEEJ";
  }
  drawCentered(footer, 127);
}

void drawFocus(uint8_t i, unsigned long now) {
  uint8_t p = percent(i);
  bool running = appRunning(i, now);
  oled.drawXBMP(24, 4, 16, 16, ICONS_16[info.icon[i]]);
  oled.setFont(u8g2_font_helvB08_tr);
  drawCentered(info.label[i], 34);
  if (info.flags[i] == FLAG_OK && !running) {
    oled.setFont(u8g2_font_5x7_tr);
    drawCentered("NOT RUNNING", 44);
  }

  if (info.flags[i] != FLAG_OK) {
    // why this knob does nothing: "N/A on LINUX" or "UNMAPPED in config"
    char line[4 + OS_MAX] = "ON ";
    bool unsupported = info.flags[i] == FLAG_UNSUPPORTED;
    oled.setFont(u8g2_font_helvB08_tr);
    drawCentered(unsupported ? "N/A" : "UNMAPPED", 64);
    oled.setFont(u8g2_font_5x7_tr);
    if (unsupported) strcat(line, info.os[0] ? info.os : "THIS OS");
    drawCentered(unsupported ? line : "IN CONFIG", 78);
    drawDottedFrame(4, 90, 56, 10);
  } else {
    if (p == 0) {
      oled.drawXBMP(24, 50, 16, 16, ICONS_16[mutedIcon(i)]);
      oled.setFont(u8g2_font_helvB08_tr);
      drawCentered("MUTED", 80);
    } else {
      drawPercent(p, 78);
    }

    // progress bar, hatched when the app isn't running
    oled.drawRFrame(4, 90, 56, 10, 3);
    uint8_t fill = (uint16_t)52 * p / 100;
    if (fill && !running) drawHatchedBox(6, 92, fill, 6);
    else if (fill >= 6) oled.drawRBox(6, 92, fill, 6, 2);
    else if (fill) oled.drawBox(6, 92, fill, 6);
  }

  // which knob this is, left to right
  for (uint8_t k = 0; k < NUM_KNOBS; k++) {
    int16_t cx = oled.getDisplayWidth() / 2 + (k * 2 - (NUM_KNOBS - 1)) * 4;
    if (k == i) oled.drawDisc(cx, 114, 2);
    else oled.drawCircle(cx, 114, 2);
  }
}

const uint8_t *kaomojiFont(const char *s) {
  return strlen(s) <= 4 ? u8g2_font_logisoso20_tr : u8g2_font_7x14B_tr;  // short frames big
}

void nextKaomoji() {  // next frame at a new random spot, so no pixel stays lit for long
  kaomojiFrame = (kaomojiFrame + 1) % NUM_KAOMOJI;
  const char *s = KAOMOJI_FRAMES[kaomojiFrame];
  oled.setFont(kaomojiFont(s));
  int16_t w = oled.getStrWidth(s);
  int16_t asc = oled.getAscent();
  int16_t bottom = oled.getDisplayHeight() + oled.getDescent();
  kaomojiX = random(0, max(1, oled.getDisplayWidth() - w + 1));
  kaomojiY = random(asc, max(asc + 1, bottom));
}

void drawKaomoji() {
  const char *s = KAOMOJI_FRAMES[kaomojiFrame];
  oled.setFont(kaomojiFont(s));
  oled.drawStr(kaomojiX, kaomojiY, s);
}

void render(View v, unsigned long now) {
  oled.firstPage();
  do {
    switch (v) {
      case VIEW_OVERVIEW: drawOverview(now); break;
      case VIEW_FOCUS:    drawFocus(focusKnob, now); break;
      case VIEW_KAOMOJI:  drawKaomoji(); break;
      default: break;
    }
  } while (oled.nextPage());
}

// ---------------------------------------------------------------- modes
void updateMode(unsigned long now) {
  unsigned long quiet = now - lastTouchMs;
  Mode want = MODE_ACTIVE;
  if (OFF_AFTER_MS && quiet >= OFF_AFTER_MS) want = MODE_OFF;
  else if (quiet >= IDLE_AFTER_MS) want = MODE_IDLE;
  if (want == mode) return;

  mode = want;
  switch (mode) {
    case MODE_ACTIVE:
      oled.setPowerSave(0);
      oled.setContrast(CONTRAST_ACTIVE);
      break;
    case MODE_IDLE:
      oled.setPowerSave(0);
      oled.setContrast(CONTRAST_IDLE);
      kaomojiFrame = NUM_KAOMOJI - 1;    // so the first tick shows frame 0
      lastFrameMs = now - KAOMOJI_MS;
      break;
    case MODE_OFF:
      oled.setPowerSave(1);
      break;
  }
  dirty = true;
}

View currentView(unsigned long now) {
  if (mode == MODE_OFF) return VIEW_NONE;
  if (mode == MODE_IDLE) return VIEW_KAOMOJI;
  if (focusKnob >= 0 && now - focusMs < FOCUS_HOLD_MS) return VIEW_FOCUS;
  return VIEW_OVERVIEW;
}

// ---------------------------------------------------------------- main
void setup() {
  Serial.begin(SERIAL_BAUD);
  randomSeed(analogRead(A7));  // A7 is unused: its noise seeds the screensaver positions
  loadLabels();

  oled.setI2CAddress(OLED_I2C_ADDRESS * 2);
  oled.setBusClock(OLED_BUS_CLOCK);
  oled.begin();
  oled.setBitmapMode(1);  // transparent bitmaps, needed for the inverted icon
  oled.setContrast(CONTRAST_ACTIVE);

  for (uint8_t i = 0; i < NUM_KNOBS; i++) {
    uint16_t raw = readKnob(KNOB_PINS[i]);
    filtered[i] = raw << 4;
    value[i] = touchRef[i] = raw;
  }

  oled.firstPage();
  do { drawSplash(); } while (oled.nextPage());
  unsigned long t0 = millis();
  while (millis() - t0 < SPLASH_MS) {  // let the filters settle behind the splash
    unsigned long t = millis();
    sampleKnobs(t);
    readHost(t);
    delay(SAMPLE_MS);
  }
  for (uint8_t i = 0; i < NUM_KNOBS; i++) touchRef[i] = value[i];  // settled start, no touch
  focusKnob = -1;
  lastTouchMs = millis();
}

void loop() {
  unsigned long now = millis();

  readHost(now);
  bool link = linkUp(now);
  if (link != linkShown) {  // deej appeared or went away
    linkShown = link;
    dirty = true;
  }

  if (now - lastSampleMs >= SAMPLE_MS) {
    lastSampleMs = now;
    if (sampleKnobs(now)) lastTouchMs = focusMs = now;
  }

  updateMode(now);
  if (mode == MODE_IDLE && now - lastFrameMs >= KAOMOJI_MS) {
    lastFrameMs = now;
    nextKaomoji();
    dirty = true;
  }

  View v = currentView(now);
  if (v != shownView) dirty = true;
  if (v == VIEW_NONE) {
    shownView = v;
    dirty = false;
  } else if (dirty && now - lastDrawMs >= FRAME_MS) {
    render(v, now);
    shownView = v;
    lastDrawMs = now;
    dirty = false;
  }

  // deej: send on change plus a heartbeat, only when it fits in the TX buffer (never blocks)
  if ((valuesChanged || now - lastSendMs >= HEARTBEAT_MS) && Serial.availableForWrite() >= LINE_MAX) {
    sendValues();
    valuesChanged = false;
    lastSendMs = now;
  }
}
