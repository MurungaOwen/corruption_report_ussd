"""for use who proceed with english"""
from ..calculations import verify_official, track_report, report_incident


def english_menu():
    return "CON Welcome to the Anti-Corruption USSD System\n1. Verify Government Official\n2. Report Corruption\n3. Track Report Status"

def english_flow(text, phone_number):
    """ English flow for the USSD service. """
        #english chosen
    if text == "1*":
        response = english_menu()
    elif text == "1*1":
        response = "CON Enter the government official's work ID:"
    elif text.startswith("1*1*"):
        official_id = text.split("*")[2]
        response = verify_official(official_id, "english")
    elif text == "1*2":
        response = "CON Please describe the corruption incident:"
    elif text.startswith("1*2*"):
        description = text.split("*")[2]
        report_id = report_incident(phone_number, description)
        response = f"END Corruption reported successfully. Your report ID is {report_id}."
    elif text == "1*3":
        response = "CON Enter your report ID to track status:"
    elif text.startswith("1*3*"):
        report_id = text.split("*")[2]
        response = track_report(report_id, "english")
    return response