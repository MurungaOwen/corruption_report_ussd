from calculations import verify_official, report_incident, track_report
def swahili_menu():
    """ Main menu in Swahili. """
    return "CON Karibu kwenye Mfumo wa Kupambana na Ufisadi\n1. Thibitisha Afisa wa Serikali\n2. Ripoti Ufisadi\n3. Fuatilia Hali ya Ripoti"

def swahili_flow(text, session_id, phone_number):
    """ Swahili flow for the USSD service. """
    global report_id_counter
    if text == "2":
        # Main menu in Swahili
        response = swahili_menu()
    elif text == "2*1":
        response = "CON Weka kitambulisho cha afisa wa serikali:"
    elif text.startswith("2*1*"):
        official_id = text.split("*")[2]
        response = verify_official(official_id, "swahili")
    elif text == "2*2":
        response = "CON Tafadhali elezea tukio la ufisadi:"
    elif text.startswith("2*2*"):
        description = text.split("*")[2]
        report_id = report_incident(phone_number, description)
        response = f"END Tukio la ufisadi limeripotiwa. Kitambulisho cha ripoti ni {report_id}."
    elif text == "2*3":
        response = "CON Weka kitambulisho chako cha ripoti kufuatilia hali:"
    elif text.startswith("2*3*"):
        report_id = text.split("*")[2]
        response = track_report(report_id, "swahili")
    else:
        response = "END Chaguo batili. Tafadhali jaribu tena."
    return response