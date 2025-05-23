//
// Created by lucio on 4/8/25.
//

#ifndef RPCLITE_RPC_H
#define RPCLITE_RPC_H

#include "transport.h"
#include "MsgPack.h"


#define CALL_MSG        0
#define RESP_MSG        1
#define NOTIFY_MSG      2

#define MAX_BUFFER_SIZE 1024
#define CHUNK_SIZE      32

uint8_t raw_buffer[MAX_BUFFER_SIZE] = {0};
size_t raw_buffer_fill = 0;


#ifdef DEBUG

void print_raw_buf(){

    //Serial.print("RPC buffer content");
    for (size_t i=0; i<raw_buffer_fill; i++) {
        //Serial.print(raw_buffer[i]);
        //Serial.print(" ");
    }
    //Serial.println(" ");

}

#endif

inline void flush_buffer(){
    raw_buffer_fill = 0;
}

inline bool is_empty_buffer() {
    return raw_buffer_fill == 0;
}

inline void send_msg(ITransport& transport, const MsgPack::bin_t<uint8_t>& buffer) {
    size_t size = buffer.size();
    transport.write(reinterpret_cast<const uint8_t*>(buffer.data()), size);
}

inline bool recv_msg(ITransport& transport, MsgPack::Unpacker& unpacker) {

    uint8_t temp_buffer[CHUNK_SIZE] = {0};

    size_t bytes_read = transport.read(temp_buffer, CHUNK_SIZE);

    if (bytes_read == 0){
        return false;
    }

    if (raw_buffer_fill + bytes_read > MAX_BUFFER_SIZE){
        // ERROR: trying to recover flushing the buffer
        flush_buffer();
        return false;
    }

    memcpy(raw_buffer + raw_buffer_fill, temp_buffer, bytes_read);
    raw_buffer_fill += bytes_read;

    unpacker.clear();
    return unpacker.feed(raw_buffer, raw_buffer_fill);

}

#endif //RPCLITE_RPC_H
