// This is a static implementation of the decoder manager

#ifndef RPCLITE_DECODER_MANAGER_H
#define RPCLITE_DECODER_MANAGER_H

#define RPCLITE_MAX_TRANSPORTS  3

#include <array>
#include "transport.h"
#include "decoder.h"

template<size_t MaxTransports = RPCLITE_MAX_TRANSPORTS>
class RpcDecoderManager {
public:
    // todo parametrize so the RpcDecoder returned has a user defined buffer size ?
    static RpcDecoder<>& getDecoder(ITransport& transport) {
        for (auto& entry : decoders_) {
            if (entry.transport == &transport) {
                return *entry.decoder;
            }

            if (entry.transport == nullptr) {
                entry.transport = &transport;
                // In-place construct
                entry.decoder = new (&entry.decoder_storage.instance) RpcDecoder<>(transport);
                return *entry.decoder;
            }
        }

        // No slot available — simple trap for now
        while (true);
    }

private:
    struct DecoderStorage {
        union {
            RpcDecoder<> instance;
            uint8_t raw[sizeof(RpcDecoder<>)];
        };

        DecoderStorage() {}
        ~DecoderStorage() {}
    };

    struct Entry {
        ITransport* transport = nullptr;
        RpcDecoder<>* decoder = nullptr;
        DecoderStorage decoder_storage;
    };

    static std::array<Entry, MaxTransports> decoders_;
};

// Definition of the static member
template<size_t MaxTransports>
std::array<typename RpcDecoderManager<MaxTransports>::Entry, MaxTransports> RpcDecoderManager<MaxTransports>::decoders_;

#endif //RPCLITE_DECODER_MANAGER_H