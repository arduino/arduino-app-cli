#define DEBUG
#include <Arduino_RPClite.h>

void blink_before(){
    digitalWrite(LED_BUILTIN, HIGH);
    delay(100);
    digitalWrite(LED_BUILTIN, LOW);
    delay(100);
    digitalWrite(LED_BUILTIN, HIGH);
    delay(100);
    digitalWrite(LED_BUILTIN, LOW);
    delay(100);
    digitalWrite(LED_BUILTIN, HIGH);
    delay(100);
    digitalWrite(LED_BUILTIN, LOW);
    delay(100);
}

MsgPack::Packer packer;
MsgPack::Unpacker unpacker;

void print_buf() {
    Serial.print("buf size: ");
    Serial.print(packer.size());
    Serial.print(" - ");

    for (size_t i=0; i<packer.size(); i++){
        Serial.print(packer.data()[i], HEX);
        Serial.print(" ");
    }
    Serial.println(" ");
}

void setup() {
    Serial.begin(9600);
}

void loop() {

    blink_before();
    MsgPack::arr_size_t req_sz(4);
    MsgPack::arr_size_t notify_sz(3);
    MsgPack::arr_size_t resp_sz(4);
    MsgPack::arr_size_t par_sz(2);

    // REQUEST
    packer.clear();
    packer.serialize(req_sz, 0, 1, "method", par_sz, 1.0, 2.0);
    print_buf();

    DummyTransport dummy_transport(packer.data(), packer.size());
    RpcDecoder<> decoder(dummy_transport);

    while (!decoder.packet_incoming()){
        Serial.println("Packet not ready");
        decoder.advance();
        decoder.parse_packet();
        delay(100);
    }

    if (decoder.packet_incoming()){
        Serial.print("packet incoming. type: ");
        Serial.println(decoder.packet_type());
    }

    // NOTIFICATION
    blink_before();
    packer.clear();
    packer.serialize(notify_sz, 2, "method", par_sz, 1.0, 2.0);
    print_buf();

    DummyTransport dummy_transport2(packer.data(), packer.size());
    RpcDecoder<> decoder2(dummy_transport2);

    while (!decoder2.packet_incoming()){
        Serial.println("Packet not ready");
        decoder2.advance();
        decoder2.parse_packet();
        delay(100);
    }

    if (decoder2.packet_incoming()){
        Serial.print("packet incoming. type: ");
        Serial.println(decoder2.packet_type());
    }

    // RESPONSE
    blink_before();
    packer.clear();
    MsgPack::object::nil_t nil;
    MsgPack::arr_size_t ret_sz(2);
    packer.serialize(resp_sz, 1, 1, nil, ret_sz, 3.0, 2);
    print_buf();

    DummyTransport dummy_transport3(packer.data(), packer.size());
    RpcDecoder<> decoder3(dummy_transport3);

    while (!decoder3.packet_incoming()){
        Serial.println("Packet not ready");
        decoder3.advance();
        decoder3.parse_packet();
        delay(100);
    }

    if (decoder3.packet_incoming()){
        Serial.print("packet incoming. type: ");
        Serial.println(decoder3.packet_type());
    }

    // MIXED INCOMING RESPONSE AND REQUEST
    Serial.println("-- Discard TEST --");
    blink_before();
    packer.clear();
    packer.serialize(resp_sz, 1, 1, nil, ret_sz, 3.0, 2);
    Serial.print("1st packet size: ");
    Serial.println(packer.size());
    packer.serialize(req_sz, 0, 1, "method", par_sz, 1.0, 2.0);
    Serial.print("full size: ");
    Serial.println(packer.size());
    print_buf();

    DummyTransport dummy_transport4(packer.data(), packer.size());
    RpcDecoder<> decoder4(dummy_transport4);

    while (!decoder4.packet_incoming()){
        Serial.println("Packet not ready");
        decoder4.advance();
        decoder4.parse_packet();
        delay(100);
    }

    while (decoder4.packet_incoming()){
        size_t removed = decoder4.discard_packet();
        Serial.print("Removed bytes: ");
        Serial.println(removed);
        decoder4.advance();
        decoder4.parse_packet();
    }

    Serial.println("-- END Discard TEST --");

}