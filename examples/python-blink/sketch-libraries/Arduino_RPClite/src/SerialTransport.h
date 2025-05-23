//
// Created by lucio on 4/8/25.
//

#ifndef SERIALTRANSPORT_H
#define SERIALTRANSPORT_H
#include "transport.h"

class SerialTransport: public ITransport {

    Stream* _stream;

    public:

        SerialTransport(Stream* stream): _stream(stream){}

        void begin(){}

        size_t write(const uint8_t* data, size_t size) override {

            for (size_t i=0; i<size; i++){
                _stream->write(data[i]);
            }

            return size;
        }

        size_t read(uint8_t* buffer, size_t size) override {

            size_t r_size = 0;

            while (_stream->available()){
                if (r_size == size){
                    return r_size;
                }
                buffer[r_size] = _stream->read();
                r_size++;
                // TODO the following delay is essential for GIGA to work. Is it worth making giga-specific?
                delay(1);
            }

            return r_size;

        }

        size_t read_byte(uint8_t& r) override {
            uint8_t b[1];
            if (read(b, 1) != 1){
                return 0;
            };
            r = b[0];
            return 1;
        }

};

#endif  //SERIALTRANSPORT_H