//
// Created by lucio on 4/8/25.
//

#ifndef RPCLITE_SERVER_H
#define RPCLITE_SERVER_H

#include "error.h"
#include "wrapper.h"
#include "dispatcher.h"
#include "decoder.h"
#include "decoder_manager.h"

#define MAX_CALLBACKS   100

class RPCServer {
    ITransport& transport;
    RpcDecoder<>& decoder;
    RpcFunctionDispatcher<MAX_CALLBACKS> dispatcher;

public:
    RPCServer(ITransport& t) : transport(t), decoder(RpcDecoderManager<>::getDecoder(t)) {}

    template<typename F>
    bool bind(const MsgPack::str_t& name, F&& func){
        return dispatcher.bind(name, func);
    }

    void run() {
        decoder.process();
        decoder.process_requests(dispatcher);
        delay(1);
    }

};

#endif //RPCLITE_SERVER_H
