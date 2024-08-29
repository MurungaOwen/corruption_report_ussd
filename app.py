from flask import Flask, request, Response
from flask_cors import CORS
from .languages.english import english_flow
from .languages.kiswahili import swahili_flow
from .calculations import user_sessions

app = Flask(__name__)

CORS(app)

@app.route('/ussd', methods=['POST'])
def ussd_callback():
    """handle request from ussd"""
    session_id = request.form.get('sessionId')
    phone_number = request.form.get('phoneNumber')
    print("phone number is {}".format(phone_number))

    text = request.form.get('text', "").strip()
    if text == "":
        # first screen
        response = "CON Choose your language (Chagua lugha):\n1. English\n2. Kiswahili"
    elif text == "1":
        user_sessions[session_id] = "english"
        response = english_flow()
    elif text == "2":
        user_sessions[session_id] = "swahili"
        response = swahili_flow()
    else:
        response = "END Invalid choice. Please try again."
    return Response(response, mimetype="text/plain")

if __name__ == '__main__':
    app.run(debug=True)
