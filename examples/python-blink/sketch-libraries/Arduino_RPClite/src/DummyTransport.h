//
// Created by lucio on 4/8/25.
//

#ifndef DUMMYTRANSPORT_H
#define DUMMYTRANSPORT_H
#include "transport.h"

class DummyTransport : public ITransport {
    std::vector<uint8_t> inbuf, outbuf;

public:
    size_t write(const uint8_t* data, size_t size) override {
        outbuf.insert(outbuf.end(), data, data + size);
        return size;
    }

    size_t read(uint8_t* buffer, size_t size) override {
        if (inbuf.size() < size) return 0;
        std::copy(inbuf.begin(), inbuf.begin() + size, buffer);
        inbuf.erase(inbuf.begin(), inbuf.begin() + size);
        return size;
    }

    size_t read_byte(uint8_t& r) override {
        uint8_t b[1];
        if (read(b, 1) != 1){
            return 0;
        };
        r = b[0];
        return 1;
    }

    void push_data(const std::vector<uint8_t>& data) { inbuf.insert(inbuf.end(), data.begin(), data.end()); }
    const std::vector<uint8_t>& get_output() const { return outbuf; }
};

#endif //DUMMYTRANSPORT_H
