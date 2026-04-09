# OmniSense: Advanced Perception Network

OmniSense is a comprehensive security and motion monitoring ecosystem. It features a high-performance Go-based central hub, ESP8266-powered motion sensors, and a real-time web dashboard for visualization, analytics, and system management.

## 🚀 Features

- **Real-time Monitoring:** WebSocket-driven live updates for all connected sensors.
- **Arming System:** Secure the network to trigger alerts and log security events.
- **Analytics Dashboard:**
    - **Hourly Activity:** Visualize motion patterns throughout the day.
    - **Room Distribution:** See which areas of your home are most active.
    - **Event Logs:** Persistent history of connections, disconnections, and alerts.
- **Room Assignment:** Dynamically map sensors to specific rooms (Living Room, Kitchen, etc.) directly from the UI.
- **Cross-Platform:** Responsive web dashboard with Dark Mode support.

---

## 🏗️ System Architecture

1.  **Sensors (Edge):** ESP8266 microcontrollers with PIR sensors detect movement and transmit states over WebSockets.
2.  **Hub (Core):** A Go server manages concurrent sensor connections, processes analytics, and broadcasts state changes.
3.  **Dashboard (UI):** A modern HTML5/JS interface for system control and data visualization.

---

## 🔌 Hardware Setup

### Components
- **Microcontroller:** ESP8266 (e.g., NodeMCU or Wemos D1 Mini)
- **Sensor:** PIR Motion Sensor (e.g., HC-SR501)
- **Power:** 5V Micro-USB or external power supply

### Wiring Diagram
> [!IMPORTANT]  
> Insert your wiring diagram image here (e.g., `docs/wiring.png`).

**Pinout Configuration:**
| Component | ESP8266 Pin | Notes |
| :--- | :--- | :--- |
| PIR VCC | 5V / VIN | Ensure PIR supports your voltage |
| PIR GND | GND | Common Ground |
| PIR OUT | GPIO 14 (D5) | Defined as `PIN_TO_SENSOR` in firmware |

---

## 🛠️ Installation & Setup

### 1. Hub Configuration (Server)
Ensure you have [Go](https://go.dev/) installed.
```bash
# Install dependencies
go mod download

# Start the server
go run server.go
```
The dashboard will be available at `http://localhost:8080`.

### 2. Sensor Firmware (Arduino)
1.  Open the Arduino IDE.
2.  Go to **File -> Preferences**.
3.  In the **Additional Boards Manager URLs** field, paste the following URL:
    `https://arduino.esp8266.com/stable/package_esp8266com_index.json`
4.  Go to **Tools -> Board -> Boards Manager**, search for "esp8266", and install the latest version.
5.  Open `motionSensor/motionSensor.ino` in the Arduino IDE.
6.  Install the required libraries:
    - `WebSockets` by Markus Sattler
    - `ESP8266WiFi` (part of the ESP8266 board package)
7.  Update the `ssid`, `password`, and `webSocket.begin` IP address to match your network and server location.
8.  Flash the firmware to your ESP8266.

---

## 📂 Project Structure

```text
OmniSense/
├── server.go             # Central Go Hub (Websocket logic & API)
├── index.html            # Web Dashboard (UI & Analytics)
├── go.mod/go.sum         # Go dependencies
├── icon/                 # SVG assets for the dashboard
└── motionSensor/
    └── motionSensor.ino  # ESP8266 Firmware
```

## ⚙️ Configuration

- **Alert Cooldown:** Adjustable via the Settings tab in the dashboard (default: 5s).
- **Dark Mode:** Toggleable UI preference.
- **System Identity:** Customize the "System Name" broadcasted across the network.

---

## 📄 License
This project is licensed under the MIT License - see the LICENSE file for details.
