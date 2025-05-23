//
// Created by lucio on 4/8/25.
//

#ifndef RPCLITE_SERVER_H
#define RPCLITE_SERVER_H

#include "rpc.h"
#include "error.h"
#include "wrapper.h"
#include "dispatcher.h"

#define MAX_CALLBACKS   100

class RPCServer {
    ITransport& transport;

public:
    RPCServer(ITransport& t) : transport(t) {}

    template<typename F>
    void bind(const MsgPack::str_t& name, F&& func){
        dispatcher.bind(name, func);
    }

    void loop() {

        if (!recv_msg(transport, unpacker)) return;

        int msg_type;
        int msg_id;
        MsgPack::str_t method;
        MsgPack::arr_size_t req_size;

        if (!unpacker.deserialize(req_size, msg_type, msg_id, method)){
            //Serial.println("unable to deserialize a msg received");

            for (size_t i=0; i<raw_buffer_fill; i++){
                //Serial.print(raw_buffer[i], HEX);
                //Serial.print(" ");
            }
            //Serial.println(" ");

            flush_buffer();
            return;
        } else {
            //Serial.print("calling method: ");
            //Serial.println(method);
        }

        MsgPack::arr_size_t resp_size(4);
        packer.clear();
        packer.serialize(resp_size, RESP_MSG, msg_id);

        dispatcher.call(method, unpacker, packer);
        flush_buffer();
        send_msg(transport, packer.packet());

    }

private:

    RpcFunctionDispatcher<MAX_CALLBACKS> dispatcher;
    MsgPack::Unpacker unpacker;
    MsgPack::Packer packer;

};

#endif //RPCLITE_SERVER_H
