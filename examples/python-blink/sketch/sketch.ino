#include <Arduino_RPClite.h>

#ifdef jomla
#define MSGPACKRPC Serial1
#else
#define MSGPACKRPC SerialUSB
#endif

SerialTransport transport(&MSGPACKRPC);
RPCServer server(transport);

void setup() {
    pinMode(LED_BUILTIN, OUTPUT);
    MSGPACKRPC.begin(115200);

    while (!MSGPACKRPC) {};
    delay(1000);

    RPCClient rpc(transport);
    bool res;
    rpc.call("$/reset", res);
    rpc.call("$/register", res, "add");
    rpc.call("$/register", res, "greet");
    rpc.call("$/register", res, "set_led");

    server.bind("add", add);
    server.bind("greet", greet);
    server.bind("set_led", set_led);
}

void quickBlinks() {
    for (int i=0; i<20; i++) {
        digitalWrite(LED_BUILTIN,HIGH);
        delay(100);
        digitalWrite(LED_BUILTIN,LOW);
        delay(100);
    }
}

bool set_led(bool state) {
    digitalWrite(LED_BUILTIN, state);
    return state;
}

int add(int a, int b) {
    return a+b;
}

MsgPack::str_t greet() {
    return MsgPack::str_t ("Hello Friend");
}

MsgPack::str_t loopback(MsgPack::str_t message) {
    return message;
}

void loop() {
    server.run();
}
