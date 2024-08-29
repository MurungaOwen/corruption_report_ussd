sample_officials = {
    "001": {"name": "owen", "position": "Officer", "authentic": True},
    "002": {"name": "Sydney", "position": "Inspector", "authentic": True},
    "003": {"name": "Nerea", "position": "General", "authentic": True},
    "004": {"name": "Maya", "position": "Tax officer", "authentic": True},
}


report_id_counter = 1
user_sessions = {}
reports = {}
report_id = 1

def verify_official(official_id, language):
    """ Verifies the government official's credentials. """
    official = sample_officials.get(official_id)
    if official:
        if official["authentic"]:
            if language == "english":
                return f"END {official['name']} ({official['position']}) is a verified government official."
            else:
                return f"END {official['name']} ({official['position']}) ni afisa wa serikali aliyehakikishwa."
        else:
            if language == "english":
                return f"END {official['name']} ({official['position']}) is NOT a verified official."
            else:
                return f"END {official['name']} ({official['position']}) si afisa wa serikali aliyehakikishwa."
    else:
        return "END Official not found." if language == "english" else "END Afisa hajapatikana."

def report_incident(phone_number, description):
    """ Records a corruption report and assigns a report ID. """
    global report_id_counter
    report_id = str(report_id_counter)
    reports[report_id] = {
        "phone_number": phone_number,
        "description": description,
        "status": "Pending"
    }
    report_id_counter += 1
    return report_id

def track_report(report_id, language):
    """ Tracks the status of a corruption report by its ID. """
    report = reports.get(report_id)
    if report:
        if language == "english":
            return f"END Report {report_id} status: {report['status']}."
        else:
            return f"END Hali ya ripoti {report_id}: {report['status']}."
    else:
        return "END Report not found." if language == "english" else "END Ripoti haijapatikana."