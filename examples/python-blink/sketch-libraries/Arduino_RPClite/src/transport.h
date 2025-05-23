//
// Created by lucio on 4/8/25.
//

#ifndef RPCLITE_TRANSPORT_H
#define RPCLITE_TRANSPORT_H

#include <Arduino.h>

class ITransport {
// Transport abstraction interface

public:
    virtual size_t write(const uint8_t* data, size_t size) = 0;
    virtual size_t read(uint8_t* buffer, size_t size) = 0;
    virtual size_t read_byte(uint8_t& r) = 0;
    //virtual ~ITransport() = default;
};

#endif //RPCLITE_TRANSPORT_H
