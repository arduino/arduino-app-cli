#include <Arduino_RPClite.h>

void blink_before(){
    digitalWrite(LED_BUILTIN, HIGH);
    delay(200);
    digitalWrite(LED_BUILTIN, LOW);
    delay(200);
    digitalWrite(LED_BUILTIN, HIGH);
    delay(200);
    digitalWrite(LED_BUILTIN, LOW);
    delay(200);
    digitalWrite(LED_BUILTIN, HIGH);
    delay(200);
    digitalWrite(LED_BUILTIN, LOW);
    delay(200);
}

MsgPack::Packer packer;
MsgPack::Unpacker unpacker;

void setup() {
    Serial.begin(9600);
}

void loop() {

    size_t expected_index_size = 5;
    blink_before();
    MsgPack::arr_size_t req_sz(4);
    MsgPack::arr_size_t par_sz(2);
    packer.clear();
    packer.serialize(req_sz, 0, 1, "method", par_sz, 1.0, 2.0);

    Serial.print("packet size: ");
    Serial.println(packer.size());

    for (size_t i=1; i< packer.size(); i++){
        unpacker.clear();
        if (unpacker.feed(packer.data(), i) && unpacker.size() >= expected_index_size){
            Serial.println("problem ");
            Serial.println(i);
        }
    }

}