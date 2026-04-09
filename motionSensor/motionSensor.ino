#include <ESP8266WiFi.h>
#include <WiFiClientSecure.h>
#include <WebSocketsClient.h>

const char* ssid = "YOUR_SSID"; // Replace with your network SSID
const char* password = "YOUR_PASSWORD"; // Replace with your network password
WebSocketsClient webSocket;

const int PIN_TO_SENSOR = 14;   // the pin that OUTPUT pin of sensor is connected to
int pinStateCurrent   = LOW; // current state of pin
int pinStatePrevious  = LOW; // previous state of pin
const unsigned long DELAY_TIME_MS = 5000;
bool delayEnabled = false;
bool notified_motion = false;
unsigned long delayStartTime;
const char* urlParams = "/?type=sensor";

void setup() {
  Serial.begin(115200);
  pinMode(PIN_TO_SENSOR, INPUT);
  WiFi.begin(ssid, password);

  while (WiFi.status() != WL_CONNECTED) {
    delay(1000);
    Serial.println("Connecting to WiFi...");
  }
  Serial.println("Connected to WiFi");

  webSocket.begin("YOUR_SERVER_IP", 8080, urlParams);
  webSocket.enableHeartbeat(5000, 3000, 2); // Send ping every 5s, 3s timeout, 2 retries
  webSocket.onEvent(webSocketEvent);
}

void loop() {
  webSocket.loop();
  pinStatePrevious = pinStateCurrent; // store state
  pinStateCurrent = digitalRead(PIN_TO_SENSOR);   // read new state

  if (pinStatePrevious == LOW && pinStateCurrent == HIGH) {   // pin state change: LOW -> HIGH
    Serial.println("Motion detected!");
    Serial.println("Turning on / activating");
    delayEnabled = false; // disable delay
    webSocket.sendTXT("motion");
  }
  else
  if (pinStatePrevious == HIGH && pinStateCurrent == LOW) {   // pin state change: HIGH -> LOW
    Serial.println("Motion stopped!");
    delayEnabled = true; // enable delay
    delayStartTime = millis(); // set start time
  }

  if (delayEnabled == true && (millis() - delayStartTime) >= DELAY_TIME_MS) {
    Serial.println("Turning off / deactivating");
    delayEnabled = false; // disable delay
    webSocket.sendTXT("!motion");
  }
}


//
void webSocketEvent(WStype_t type, uint8_t * payload, size_t length) {
    switch(type) {
        case WStype_DISCONNECTED:
            Serial.printf("[WSc] Disconnected!\n");
            break;
        case WStype_CONNECTED:
            Serial.printf("[WSc] Connected to url: %s\n", payload);
            webSocket.sendTXT("pong");
            break;
        case WStype_TEXT:
            Serial.printf("[WSc] get text: %s\n", payload);
            // Send pong response if the message is "ping"
            if (strcmp((char*)payload, "ping") == 0) {
                webSocket.sendTXT("pong");
            }
            break;
        case WStype_BIN:
            Serial.printf("[WSc] get binary length: %u\n", length);
            break;
        case WStype_ERROR:
        case WStype_FRAGMENT_TEXT_START:
        case WStype_FRAGMENT_BIN_START:
        case WStype_FRAGMENT:
        case WStype_FRAGMENT_FIN:
            break;
    }
}
