#ifndef RPCLITE_WRAPPER_H
#define RPCLITE_WRAPPER_H

#include "error.h"

#ifdef HANDLE_RPC_ERRORS
#include <stdexcept>
#endif

//TODO maybe use arx::function_traits

// C++11-compatible function_traits
// Primary template: fallback
template<typename T>
struct function_traits;

// Function pointer
template<typename R, typename... Args>
struct function_traits<R(*)(Args...)> {
    using signature = R(Args...);
};

// std::function
template<typename R, typename... Args>
struct function_traits<std::function<R(Args...)>> {
    using signature = R(Args...);
};

// Member function pointer (including lambdas)
template<typename C, typename R, typename... Args>
struct function_traits<R(C::*)(Args...) const> {
    using signature = R(Args...);
};

// Deduction helper for lambdas
template<typename T>
struct function_traits {
    using signature = typename function_traits<decltype(&T::operator())>::signature;
};


// Helper to invoke a function with a tuple of arguments
template<typename F, typename Tuple, std::size_t... I>
auto invoke_with_tuple(F&& f, Tuple&& t, arx::stdx::index_sequence<I...>)
    -> decltype(f(std::get<I>(std::forward<Tuple>(t))...)) {
    return f(std::get<I>(std::forward<Tuple>(t))...);
};

template<typename F>
class RpcFunctionWrapper;

class IFunctionWrapper {
    public:
        virtual ~IFunctionWrapper() {}
        virtual bool operator()(MsgPack::Unpacker& unpacker, MsgPack::Packer& packer) = 0;
    };

template<typename R, typename... Args>
class RpcFunctionWrapper<R(Args...)>: public IFunctionWrapper {
public:
    RpcFunctionWrapper(std::function<R(Args...)> func) : _func(func) {}

    R operator()(Args... args) {
        return _func(args...);
    }

    bool operator()(MsgPack::Unpacker& unpacker, MsgPack::Packer& packer) override {

        MsgPack::object::nil_t nil;

#ifdef HANDLE_RPC_ERRORS
    try {
#endif

        // First check the parameters size
        if (!unpacker.isArray()){
            RpcError error(MALFORMED_CALL_ERR, "Unserializable parameters array");
            packer.serialize(error, nil);
            return false;
        }

        MsgPack::arr_size_t param_size;

        unpacker.deserialize(param_size);
        if (param_size.size() < sizeof...(Args)){
            RpcError error(MALFORMED_CALL_ERR, "Missing call parameters (WARNING: Default param resolution is not implemented)");
            packer.serialize(error, nil);
            return false;
        }

        if (param_size.size() > sizeof...(Args)){
            RpcError error(MALFORMED_CALL_ERR, "Too many parameters");
            packer.serialize(error, nil);
            return false;
        }

        auto args = deserialize_all<Args...>(unpacker);
        if constexpr (std::is_void<R>::value){
            invoke_with_tuple(_func, args, arx::stdx::make_index_sequence<sizeof...(Args)>{});
            packer.serialize(nil, nil);
            return true;
        } else {
            R out = invoke_with_tuple(_func, args, arx::stdx::make_index_sequence<sizeof...(Args)>{});
            //Serial.println(out);
            packer.serialize(nil, out);
            return true;
        }
#ifdef HANDLE_RPC_ERRORS
    } catch (const std::exception& e) {
        // Should be specialized according to the exception type
        RpcError error(GENERIC_ERR, "RPC error");
        packer.serialize(error, nil);
        return false;
    }
#endif

    }

private:
    std::function<R(Args...)> _func;

    template<typename... Ts>
    std::tuple<Ts...> deserialize_all(MsgPack::Unpacker& unpacker) {
        return std::make_tuple(deserialize_single<Ts>(unpacker)...);
    }

    template<typename T>
    static T deserialize_single(MsgPack::Unpacker& unpacker) {
        T value;
        unpacker.deserialize(value);
        return value;
    }
};


template<typename F>
auto wrap(F&& f) -> RpcFunctionWrapper<typename function_traits<typename std::decay<F>::type>::signature> {
    using Signature = typename function_traits<typename std::decay<F>::type>::signature;
    return RpcFunctionWrapper<Signature>(std::forward<F>(f));
};

#endif