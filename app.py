from flask import Flask, request, Response
from flask_cors import CORS
from languages.english import english_flow
from languages.kiswahili import swahili_flow
from  calculations import user_sessions

app = Flask(__name__)

CORS(app)

@app.route('/ussd', methods=['POST'])
def ussd_callback():
    """handle request from ussd"""
    session_id = request.form.get('sessionId')
    phone_number = request.form.get('phoneNumber')

    text = request.form.get('text', "").strip()
    app.logger.info(f"Connected user: {phone_number} : session_ID: {session_id}: text: {text}")
    
    if text == "":
        # first screen
        response = "CON Choose your language (Chagua lugha):\n1. English\n2. Kiswahili"
    elif text.startswith("1"):
        user_sessions[session_id] = "english"
        response = english_flow(text, phone_number)
    elif text.startswith("2"):
        user_sessions[session_id] = "swahili"
        response = swahili_flow(text, phone_number)
    else:
        response = "END Invalid choice. Please try again."
    return Response(response, mimetype="text/plain")

if __name__ == '__main__':
    app.run(host="0.0.0.0",debug=True)