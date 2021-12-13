import React, { useCallback, useState, createContext } from 'react';
import Toast from 'react-bootstrap/Toast';

const ToasterContext = createContext();

export const ToasterContextProvider = ({children}) => {

	const [toasts, setToasts] = useState([]);

	const addToast = useCallback((toast) => {
			toast.id = Math.floor((Math.random() * 10001) + 1);
			toast.autohide = toast.autohide || 0;

			// Don't display more than 10 messages
			setToasts((t) => {
				if (t.length > 9) {
					t.shift();
				}
				return [...t, toast];
			});
		},
		[]
	);

	const deleteToast = (id) => {
		setToasts((t) => t.filter(e => e.id !== id));
	};

	return (
		<ToasterContext.Provider value={addToast}>
			{children}
			<div className={"toasters-container"} >
				{ toasts.map((toast) => (
					<Toast
						key={toast.id}
						onClose={() => deleteToast(toast.id)}
						className={"toaster-"+toast.type}
						{... (toast.autohide > 0 ? {delay: toast.autohide, autohide: true} : {})}
					>
					  <Toast.Header>
						<strong className="mr-auto">{toast.title}</strong>
					  </Toast.Header>
					  <Toast.Body>{toast.msg}</Toast.Body>
					</Toast>
				  ))
				}
			</div>
		</ToasterContext.Provider>
	);
}

export default ToasterContext;
