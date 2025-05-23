from arduino.app_bricks.mcu import call, register
import time

#curr_led_state = False
#@register()
#def get_led_state():
#    global curr_led_state
#    curr_led_state = not curr_led_state
#    return curr_led_state

@call()
def set_led(state: bool) -> bool:
    ...

# Sleep to keep the RPC server alive
print("waiting for requests...")
while True:
    try:
        set_led(True)
    except Exception as e:
        pass
    time.sleep(1)
    try:
        set_led(False)
    except Exception as e:
        pass
    time.sleep(1)
