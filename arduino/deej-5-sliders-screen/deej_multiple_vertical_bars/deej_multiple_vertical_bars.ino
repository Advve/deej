#include <Wire.h>
#include <Adafruit_GFX.h>
#include <Adafruit_SSD1306.h>

#define SCREEN_WIDTH 128 // OLED display width, in pixels
#define SCREEN_HEIGHT 64 // OLED display height, in pixels

// Declaration for an SSD1306 display connected to I2C (SDA, SCL pins A4 A5)
Adafruit_SSD1306 display(SCREEN_WIDTH, SCREEN_HEIGHT, &Wire, -1);

const int NUM_SLIDERS = 5;
int gap = 5; // Gap between beams
int beam = round((128 - ((NUM_SLIDERS + 1) * gap)) / NUM_SLIDERS); // Horizontal length of a beam

const int analogInputs[NUM_SLIDERS] = {A0, A1, A2, A3, A6};

int analogSliderValues[NUM_SLIDERS];
int previousSliderValues[NUM_SLIDERS]; // Store previous values
const int DEADBAND = 15; // Margin of error to ignore small changes
const unsigned long IDLE_TIMEOUT = 5000; // Time in milliseconds to switch to idle mode
unsigned long lastActiveTime = 0; // Tracks the last time sliders were active

void setup() {
  display.begin(SSD1306_SWITCHCAPVCC, 0x3C);
  display.clearDisplay();
  display.setTextColor(WHITE);
  delay(500);
  display.clearDisplay();

  for (int i = 0; i < NUM_SLIDERS; i++) {
    pinMode(analogInputs[i], INPUT);
    previousSliderValues[i] = 0; // Initialize previous values
  }

  lastActiveTime = millis(); // Initialize last active time
  Serial.begin(9600);
}

void loop() {
  if (checkIfActive()) {
    sendSliderValues(); // Show bars when active
  } else {
    showKaomojiAnimation(); // Show kaomoji when idle
  }
  delay(10);
}

bool checkIfActive() {
  bool isActive = false;

  for (int i = 0; i < NUM_SLIDERS; i++) {
    int rawValue = analogRead(analogInputs[i]);

    // Check activity based on DEADBAND
    if (abs(rawValue - previousSliderValues[i]) > DEADBAND) {
      previousSliderValues[i] = rawValue; // Update previous values for activity tracking
      isActive = true;
    }

    // Update analogSliderValues with DEADBAND logic applied
    if (abs(rawValue - analogSliderValues[i]) > DEADBAND) {
      analogSliderValues[i] = rawValue;
    }
  }

  if (isActive) {
    lastActiveTime = millis();
  }

  // Return true if within active time window
  return (millis() - lastActiveTime) < IDLE_TIMEOUT;
}


void sendSliderValues() {
  display.clearDisplay();
  String builtString = String("");

  for (int i = 0; i < NUM_SLIDERS; i++) {
    // Flip slider index for display order
    int flippedIndex = NUM_SLIDERS - 1 - i;

    // Use raw values for serial output
    int rawValue = analogRead(analogInputs[flippedIndex]);
    builtString += String(rawValue);

    if (i < NUM_SLIDERS - 1) {
      builtString += String("|");
    }

    // Map smoothed (DEADBAND-affected) values for bar display
    int currentval = map(analogSliderValues[flippedIndex], 0, 1023, 64, 0);

    // Draw bar outline and fill the bar with correct orientation
    display.drawRect(beam * i + gap * (i + 1), 0, beam, 64, WHITE);
    display.fillRect(beam * i + gap * (i + 1), 64 - currentval, beam, currentval, WHITE);
  }

  display.display();
  Serial.println(builtString); // Print raw values to Serial
}

void showKaomojiAnimation() {
  static int frame = 0;
  static unsigned long lastFrameTime = 0; // Tracks the last time the frame was updated
  const unsigned long frameInterval = 1000; // Time interval between frames

  // Define kaomoji frames with more variety
  const char *kaomojiFrames[] = {
      "(=^_^=)",
      "(=^-^=)",
      "(=^w^=)",
      "(^._.^)",
      "(=^o^=)",
      "(^OwO^)",
      "(^UwU^)",
      "(=^.^=)",
      "A",
      "AD",
      "ADV",
      "ADVE"
  };

  int totalFrames = sizeof(kaomojiFrames) / sizeof(kaomojiFrames[0]);

  // Update kaomoji frame only if enough time has passed
  if (millis() - lastFrameTime >= frameInterval) {
    lastFrameTime = millis(); // Update the last frame time
    frame = (frame + 1) % totalFrames; // Cycle to the next frame

    display.clearDisplay();
    display.setTextSize(3); // Increase text size for larger kaomoji
    display.setCursor((SCREEN_WIDTH - 18 * strlen(kaomojiFrames[frame])) / 2, (SCREEN_HEIGHT - 24) / 2);
    display.print(kaomojiFrames[frame]);
    display.display();
  }
}
