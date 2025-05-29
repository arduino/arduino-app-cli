#include <Arduino_RPClite.h>

#ifdef jomla
extern "C" void matrixWrite(const uint32_t* buf);
#else
#include <Arduino_LED_Matrix.h>
ArduinoLEDMatrix matrix;
void matrixWrite(const uint32_t* buf) {
  matrix.loadFrame(buf);
}
#endif

#include "weather_frames.h"

#ifdef jomla
#define MSGPACKRPC Serial1
#else
#define MSGPACKRPC SerialUSB
#endif

SerialTransport transport(&MSGPACKRPC);
RPCClient rpc(transport);

void setup() {
  MSGPACKRPC.begin(115200);

#ifndef jomla
  matrix.begin();
#endif
}

String city = "Turin";
unsigned long prevMillis = 0;
void loop() {
  unsigned long currMillis = millis();
  if (currMillis - prevMillis >= 10000) {  // 10 seconds
    prevMillis = currMillis;
    String weather_forecast;
    bool ok = rpc.call("get_weather_forecast", weather_forecast, city);
    if (ok) {
      if (weather_forecast == "sunny") {
        matrixWrite(sunny);
      } else if (weather_forecast == "cloudy") {
        matrixWrite(cloudy);
      } else if (weather_forecast == "rainy") {
        matrixWrite(rainy);
      } else if (weather_forecast == "snowy") {
        matrixWrite(snowy);
      } else if (weather_forecast == "foggy") {
        matrixWrite(foggy);
      }
    }
  }
}
